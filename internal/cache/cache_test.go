package cache

import (
	"testing"

	"github.com/shanehull/markethunt/internal/model"
)

func TestPutAndSnapshot(t *testing.T) {
	c := New()
	if c.Snapshot() != nil {
		t.Fatal("new store should be empty")
	}
	snap := &model.Snapshot{Countries: []model.Country{{ISO3: "AUS"}}}
	c.Put(snap)
	if c.Snapshot() != snap {
		t.Fatal("snapshot not stored")
	}
	next := &model.Snapshot{Countries: []model.Country{{ISO3: "JPN"}}}
	c.Put(next)
	if c.Snapshot() != next {
		t.Fatal("snapshot not replaced")
	}
}
