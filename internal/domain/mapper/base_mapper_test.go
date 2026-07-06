package mapper

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/itsLeonB/go-crud"
	"github.com/stretchr/testify/assert"
)

func TestBaseToDTO(t *testing.T) {
	now := time.Now().Truncate(time.Second)
	id := uuid.New()
	be := crud.BaseEntity{ID: id, CreatedAt: now, UpdatedAt: now.Add(time.Hour)}

	got := BaseToDTO(be)

	assert.Equal(t, id, got.ID)
	assert.Equal(t, now, got.CreatedAt)
	assert.Equal(t, now.Add(time.Hour), got.UpdatedAt)
}
