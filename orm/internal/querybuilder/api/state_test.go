package api

import (
	"reflect"
	"testing"

	"github.com/recoweft/goquent/orm/internal/querybuilder/internal/db/base"
)

func TestBaseBuildSnapshotCorrespondence(t *testing.T) {
	b := NewSelectQueryBuilder(base.NewBaseQueryBuilder()).Table("users").Where("id", "=", 2)
	_, args, snapshot, err := b.BuildSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.WhereTree == nil || snapshot.WhereTree.Correspondence != "generated" || !reflect.DeepEqual(args, []any{2}) || snapshot.WhereTree.Parameters[0] != 0 {
		t.Fatal("missing base snapshot trace")
	}
}
