package services

import (
	"testing"

	"github.com/nikiaprian/tirtayasa/internal/entity"
)

func TestNormalizeCategoryParentID(t *testing.T) {
	zero := uint(0)
	c := &entity.Category{ParentID: &zero}
	normalizeCategoryParentID(c)
	if c.ParentID != nil {
		t.Fatal("expected nil parent")
	}
}
