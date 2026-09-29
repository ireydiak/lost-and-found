package triage

import (
	"errors"
	"testing"
)

func TestSaveTags_NoTagsSelected(t *testing.T) {
	// SaveTags must reject an empty tag list before ever touching the
	// database -- passing a nil *sql.DB here would panic if that guard
	// were ever removed or reordered, which is exactly what makes this a
	// useful regression test.
	err := SaveTags(nil, "somehash", nil)
	if !errors.Is(err, ErrNoTagsSelected) {
		t.Fatalf("expected ErrNoTagsSelected, got %v", err)
	}

	err = SaveTags(nil, "somehash", []int64{})
	if !errors.Is(err, ErrNoTagsSelected) {
		t.Fatalf("expected ErrNoTagsSelected for empty slice, got %v", err)
	}
}
