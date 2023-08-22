package validator_test

import (
	"testing"

	"example.com/gofurther/internal/validator"

	"github.com/stretchr/testify/assert"
)

func TestUnique(t *testing.T) {
	tests := []struct {
		name  string
		given []string
		want  bool
	}{
		{
			name:  "Empty",
			given: []string{},
			want:  true,
		},
		{
			name:  "2 uniques",
			given: []string{"one", "two"},
			want:  true,
		},
		{
			name:  "2 non replicas",
			given: []string{"one", "one"},
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, validator.Unique(tt.given), tt.want, "they should be equal")
		})
	}
}
