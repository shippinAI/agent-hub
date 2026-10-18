package tools

import (
	"context"
	"errors"
	"os"
	"syscall"

	"devagotchi/bridge/fsops"
	"devagotchi/bridge/policy"
)

type moveIn struct {
	Source       string `json:"source"`
	Destination  string `json:"destination"`
	Overwrite    bool   `json:"overwrite,omitempty" jsonschema:"replace an existing destination file (default: refuse)"`
	ExpectSHA256 string `json:"expect_sha256,omitempty" jsonschema:"refuse if the source file changed"`
}

func moveFile() Tool {
	return def(Tool{Name: "move_file", Title: "Move/Rename File", Cap: policy.CapWrite, Destructive: true,
		Desc: "Move or rename a file or directory. Source and destination must both be inside the write roots. An existing " +
			"destination is never clobbered unless overwrite is true. Works across filesystems (copy + verify + remove for files)."},
		func(ctx context.Context, e *Env, in moveIn) Out {
			src, err := e.Gate.Path(policy.CapWrite, in.Source)
			if err != nil {
				return errOut(err)
			}
			dst, err := e.Gate.Path(policy.CapWrite, in.Destination)
			if err != nil {
				return errOut(err)
			}
			if _, err := os.Lstat(src); err != nil {
				return errOut(err)
			}
			if in.ExpectSHA256 != "" && fileSHA(src, e.Gate.Limits().MaxReadBytes) != in.ExpectSHA256 {
				return fail("hash_mismatch", "the source changed since you read it")
			}
			if _, err := os.Lstat(dst); err == nil && !in.Overwrite {
				return fail("exists", "%s exists; pass overwrite=true to replace it", dst)
			}
			err = os.Rename(src, dst)
			if errors.Is(err, syscall.EXDEV) {
				err = crossMove(src, dst, e.Gate.Limits().MaxReadBytes)
			}
			if err != nil {
				return errOut(err)
			}
			return ok("Moved "+src+" to "+dst, map[string]any{"source": src, "destination": dst})
		})
}

func crossMove(src, dst string, limit int64) error {
	b, err := readCapped(src, limit)
	if err != nil {
		return err
	}
	if err := fsops.AtomicWrite(dst, b); err != nil {
		return err
	}
	if fileSHA(dst, limit) != shaOf(b) {
		return errors.New("copy verification failed; source kept")
	}
	return os.Remove(src)
}

type deleteIn struct {
	Path         string `json:"path"`
	ExpectSHA256 string `json:"expect_sha256" jsonschema:"required: sha256 of the file from read_file or get_file_info"`
}

func deleteFile() Tool {
	return def(Tool{Name: "delete_file", Title: "Delete File (hash-checked)", Cap: policy.CapWrite, Destructive: true,
		Desc: "Delete one file. Requires expect_sha256 of its current content, so only the version you inspected can be " +
			"deleted. Directories are not deleted by this tool."},
		func(ctx context.Context, e *Env, in deleteIn) Out {
			p, err := e.Gate.Path(policy.CapWrite, in.Path)
			if err != nil {
				return errOut(err)
			}
			if in.ExpectSHA256 == "" {
				return fail("invalid", "expect_sha256 is required")
			}
			if cur := fileSHA(p, 1<<40); cur != in.ExpectSHA256 {
				return Out{Code: "hash_mismatch", Text: "Error (hash_mismatch): the file is not the version you named", Data: map[string]any{"current": cur}}
			}
			if err := os.Remove(p); err != nil {
				return errOut(err)
			}
			return ok("Deleted "+p, map[string]any{"path": p})
		})
}
