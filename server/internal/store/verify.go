package store

import "context"

// ChainReport is the result of re-hashing the whole log.
type ChainReport struct {
	OK        bool   `json:"ok"`
	Count     int64  `json:"count"`
	HeadSeq   int64  `json:"head_seq"`
	HeadHash  string `json:"head_hash"`
	BrokenSeq int64  `json:"broken_seq,omitempty"`
	Reason    string `json:"reason,omitempty"`
}

// VerifyChain recomputes every hash from genesis and checks seq continuity.
func (s *Store) VerifyChain(ctx context.Context) (ChainReport, error) {
	rows, err := s.DB.Query(ctx, `SELECT seq,at,actor,action,task_id,data::text,fence,request_id,prev_hash,hash FROM events ORDER BY seq`)
	if err != nil {
		return ChainReport{}, err
	}
	defer rows.Close()
	rep := ChainReport{OK: true}
	prev, want := genesis, int64(1)
	for rows.Next() {
		var e Event
		var data string
		if err := rows.Scan(&e.Seq, &e.At, &e.Actor, &e.Action, &e.TaskID, &data, &e.Fence, &e.RequestID, &e.PrevHash, &e.Hash); err != nil {
			return rep, err
		}
		c, err := canonical([]byte(data))
		if err != nil {
			return rep, err
		}
		e.Data = c
		bad := ""
		switch {
		case e.Seq != want:
			bad = "sequence gap"
		case e.PrevHash != prev:
			bad = "prev_hash does not link to the previous row"
		case eventHash(prev, e) != e.Hash:
			bad = "row content does not match its hash"
		}
		if bad != "" && rep.OK {
			rep.OK, rep.BrokenSeq, rep.Reason = false, e.Seq, bad
		}
		rep.Count, rep.HeadSeq, rep.HeadHash = rep.Count+1, e.Seq, e.Hash
		prev, want = e.Hash, e.Seq+1
	}
	return rep, rows.Err()
}
