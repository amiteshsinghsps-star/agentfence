package audit

import (
	"context"
	"testing"
)

func TestAppendAndVerify(t *testing.T) {
	c := NewChain()
	ctx := context.Background()

	_, err := c.Append(ctx, "call-1", "sess-1", "fs.read", "ALLOW", "low risk", 0.1)
	if err != nil {
		t.Fatalf("failed to append first record: %v", err)
	}

	_, err = c.Append(ctx, "call-2", "sess-1", "fs.write", "DENY", "scope violation", 0.6)
	if err != nil {
		t.Fatalf("failed to append second record: %v", err)
	}

	_, err = c.Append(ctx, "call-3", "sess-2", "kubectl.delete", "REQUIRE_APPROVAL", "destructive", 0.3)
	if err != nil {
		t.Fatalf("failed to append third record: %v", err)
	}

	valid, err := c.VerifyChain(ctx)
	if err != nil {
		t.Fatalf("chain verification error: %v", err)
	}
	if !valid {
		t.Fatal("expected chain to be valid")
	}
}

func TestAppendRecordCount(t *testing.T) {
	c := NewChain()
	ctx := context.Background()

	for i := 0; i < 10; i++ {
		c.Append(ctx, "call-1", "sess-1", "fs.read", "ALLOW", "test", 0.1)
	}

	if count := c.GetRecordCount(); count != 10 {
		t.Errorf("expected 10 records, got %d", count)
	}
}

func TestVerifyRange(t *testing.T) {
	c := NewChain()
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		c.Append(ctx, "call-1", "sess-1", "fs.read", "ALLOW", "test", 0.1)
	}

	valid, err := c.VerifyRange(ctx, 0, 4)
	if err != nil {
		t.Fatalf("range verification error: %v", err)
	}
	if !valid {
		t.Fatal("expected range to be valid")
	}
}

func TestExportNDJSON(t *testing.T) {
	c := NewChain()
	ctx := context.Background()

	for i := 0; i < 3; i++ {
		c.Append(ctx, "call-1", "sess-1", "fs.read", "ALLOW", "test", 0.1)
	}

	export, err := c.ExportNDJSON(ctx, 0, 2)
	if err != nil {
		t.Fatalf("export error: %v", err)
	}

	if len(export) == 0 {
		t.Fatal("expected non-empty export")
	}
}

func TestHashChainOrdering(t *testing.T) {
	c := NewChain()
	ctx := context.Background()

	r1, _ := c.Append(ctx, "call-1", "sess-1", "fs.read", "ALLOW", "first", 0.1)
	r2, _ := c.Append(ctx, "call-2", "sess-1", "fs.write", "DENY", "second", 0.5)

	if r1.PrevHash != "GENESIS" {
		t.Errorf("first record should have GENESIS prev_hash, got %s", r1.PrevHash)
	}
	if r2.PrevHash != r1.ThisHash {
		t.Errorf("second record prev_hash should equal first this_hash: %s != %s", r2.PrevHash, r1.ThisHash)
	}
}
