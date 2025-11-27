package main

import (
	"testing"

	"github.com/google/uuid"
	openapi_types "github.com/oapi-codegen/runtime/types"
)

func TestStack_Stackable(t *testing.T) {
	id1 := uuid.New()
	id2 := uuid.New()
	tests := []struct {
		name  string
		stack Stack
		want  bool
	}{
		{
			name: "stackable with parent and IDs",
			stack: Stack{
				IDs:    []openapi_types.UUID{id1},
				Parent: &id2,
			},
			want: true,
		},
		{
			name: "not stackable without parent",
			stack: Stack{
				IDs: []openapi_types.UUID{id1},
			},
			want: false,
		},
		{
			name: "not stackable without IDs",
			stack: Stack{
				Parent: &id1,
			},
			want: false,
		},
		{
			name:  "not stackable empty",
			stack: Stack{},
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.stack.Stackable(); got != tt.want {
				t.Errorf("Stack.Stackable() = %v, want %v", got, tt.want)
			}
		})
	}
}
