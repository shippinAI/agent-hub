package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"agenthub/internal/store"
)

// auditAnchor verifies the whole hash chain and writes a dated head-hash anchor file (D11). The anchor is a
// tamper-evident checkpoint: re-running VerifyChain later and comparing proves no earlier row was altered.
// Folder comes from --dir or the audit_anchor_dir setting; nothing is written when both are empty.
func auditAnchor(ctx context.Context, st *store.Store, args []string) error {
	fs := flag.NewFlagSet("anchor", flag.ContinueOnError)
	dir := fs.String("dir", "", "output folder (defaults to the audit_anchor_dir setting)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	rep, err := st.VerifyChain(ctx)
	if err != nil {
		return err
	}
	if !rep.OK {
		return fmt.Errorf("audit chain BROKEN at seq %d: %s (refusing to anchor)", rep.BrokenSeq, rep.Reason)
	}
	out := *dir
	if out == "" {
		set, _ := st.LoadSettings(ctx)
		out = set.String("audit_anchor_dir", "")
	}
	anchor := map[string]any{"at": time.Now().UTC().Format(time.RFC3339), "head_seq": rep.HeadSeq, "head_hash": rep.HeadHash, "count": rep.Count}
	b, _ := json.MarshalIndent(anchor, "", "  ")
	if out == "" {
		fmt.Printf("audit ok: %d events, head seq %d hash %s (no audit_anchor_dir set; not written)\n", rep.Count, rep.HeadSeq, rep.HeadHash)
		return nil
	}
	if err := os.MkdirAll(out, 0o700); err != nil {
		return err
	}
	name := filepath.Join(out, "anchor-"+time.Now().UTC().Format("2006-01-02")+".json")
	if err := os.WriteFile(name, append(b, '\n'), 0o600); err != nil {
		return err
	}
	fmt.Printf("audit ok: %d events; anchor written to %s\n", rep.Count, name)
	return nil
}

var errAuditUsage = errors.New("usage: hub-server admin audit anchor [--dir DIR]")
