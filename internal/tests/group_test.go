package tests

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yunobar/album/internal/domain/dto"
	"github.com/yunobar/album/internal/domain/entity"
	"github.com/yunobar/album/internal/testhelpers"
)

func TestGroupCreate(t *testing.T) {
	testhelpers.RequireTestDB(t, testDB)

	t.Run("creates a group with no name, auto-joins the creator, derives a default name", func(t *testing.T) {
		testhelpers.TruncateAll(t, testDB)
		seedTestUser(t)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/groups", strings.NewReader(`{}`))
		req.Header.Set("Content-Type", "application/json")
		testRouter.ServeHTTP(w, req)
		require.Equal(t, http.StatusCreated, w.Code)

		var resp struct {
			Data dto.GroupResponse `json:"data"`
		}
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
		assert.Equal(t, "New group", resp.Data.Name)
		assert.NotEmpty(t, resp.Data.InviteToken)
		require.Len(t, resp.Data.Members, 1)
		assert.Equal(t, testProfileID, resp.Data.Members[0].ID)
		assert.Equal(t, "Test User", resp.Data.Members[0].Name)

		var members []entity.GroupMember
		require.NoError(t, testDB.Find(&members).Error)
		require.Len(t, members, 1)
		assert.Equal(t, testProfileID, members[0].ProfileID)
	})

	t.Run("creates a named group", func(t *testing.T) {
		testhelpers.TruncateAll(t, testDB)
		seedTestUser(t)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/groups", strings.NewReader(`{"name":"Movie Night"}`))
		req.Header.Set("Content-Type", "application/json")
		testRouter.ServeHTTP(w, req)
		require.Equal(t, http.StatusCreated, w.Code)

		var resp struct {
			Data dto.GroupResponse `json:"data"`
		}
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
		assert.Equal(t, "Movie Night", resp.Data.Name)
	})
}

func TestGroupGet(t *testing.T) {
	testhelpers.RequireTestDB(t, testDB)

	t.Run("returns detail for a member, deriving the name from the other members", func(t *testing.T) {
		testhelpers.TruncateAll(t, testDB)
		seedTestUser(t)
		bob := seedTestProfile(t, "Bob")
		group := seedTestGroup(t, nil, testProfileID, bob.ID)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/api/v1/groups/"+group.ID.String(), nil)
		testRouter.ServeHTTP(w, req)
		require.Equal(t, http.StatusOK, w.Code)

		var resp struct {
			Data dto.GroupResponse `json:"data"`
		}
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
		assert.Equal(t, "Bob", resp.Data.Name)
		assert.Len(t, resp.Data.Members, 2)
	})

	t.Run("404s for a non-member, without confirming the group exists", func(t *testing.T) {
		testhelpers.TruncateAll(t, testDB)
		seedTestUser(t)
		other := seedTestProfile(t, "Other")
		group := seedTestGroup(t, nil, other.ID)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/api/v1/groups/"+group.ID.String(), nil)
		testRouter.ServeHTTP(w, req)
		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("404s for a group ID that doesn't exist", func(t *testing.T) {
		testhelpers.TruncateAll(t, testDB)
		seedTestUser(t)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/api/v1/groups/"+uuid.New().String(), nil)
		testRouter.ServeHTTP(w, req)
		assert.Equal(t, http.StatusNotFound, w.Code)
	})
}

func TestGroupList(t *testing.T) {
	testhelpers.RequireTestDB(t, testDB)

	t.Run("lists the caller's groups with a per-viewer derived name and member count", func(t *testing.T) {
		testhelpers.TruncateAll(t, testDB)
		seedTestUser(t)
		bob := seedTestProfile(t, "Bob")
		carol := seedTestProfile(t, "Carol")
		other := seedTestProfile(t, "Not In This Group")

		g1 := seedTestGroup(t, nil, testProfileID, bob.ID, carol.ID)
		seedTestGroup(t, nil, other.ID)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/api/v1/groups", nil)
		testRouter.ServeHTTP(w, req)
		require.Equal(t, http.StatusOK, w.Code)

		var resp struct {
			Data dto.ListGroupsResponse `json:"data"`
		}
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
		require.Len(t, resp.Data.Groups, 1, "must not include a group the caller isn't a member of")
		assert.Equal(t, g1.ID, resp.Data.Groups[0].ID)
		assert.Equal(t, "Bob & Carol", resp.Data.Groups[0].Name)
		assert.Equal(t, 3, resp.Data.Groups[0].MemberCount)
	})
}

func TestGroupJoin(t *testing.T) {
	testhelpers.RequireTestDB(t, testDB)

	t.Run("joins the group addressed by the token", func(t *testing.T) {
		testhelpers.TruncateAll(t, testDB)
		seedTestUser(t)
		owner := seedTestProfile(t, "Owner")
		group := seedTestGroup(t, nil, owner.ID)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/groups/join/"+group.InviteToken, nil)
		testRouter.ServeHTTP(w, req)
		require.Equal(t, http.StatusOK, w.Code)

		var resp struct {
			Data dto.GroupResponse `json:"data"`
		}
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
		assert.Len(t, resp.Data.Members, 2)

		var members []entity.GroupMember
		require.NoError(t, testDB.Where("group_id = ?", group.ID).Find(&members).Error)
		assert.Len(t, members, 2)
	})

	t.Run("joining twice is idempotent — 200, not 409, and no duplicate row", func(t *testing.T) {
		testhelpers.TruncateAll(t, testDB)
		seedTestUser(t)
		group := seedTestGroup(t, nil, testProfileID)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/groups/join/"+group.InviteToken, nil)
		testRouter.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)

		var members []entity.GroupMember
		require.NoError(t, testDB.Where("group_id = ?", group.ID).Find(&members).Error)
		require.Len(t, members, 1, "already-a-member join must not insert a duplicate row")
	})

	t.Run("404s for an unknown token", func(t *testing.T) {
		testhelpers.TruncateAll(t, testDB)
		seedTestUser(t)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/groups/join/"+uuid.New().String(), nil)
		testRouter.ServeHTTP(w, req)
		assert.Equal(t, http.StatusNotFound, w.Code)
	})
}

// seedTestGroup creates a group (nil name derives; pass a pointer for a fixed
// name) and joins each of memberIDs to it in group_members.
func seedTestGroup(t *testing.T, name *string, memberIDs ...uuid.UUID) entity.Group {
	t.Helper()
	group := entity.Group{Name: name, InviteToken: uuid.NewString()}
	require.NoError(t, testDB.Create(&group).Error)
	for _, id := range memberIDs {
		require.NoError(t, testDB.Create(&entity.GroupMember{GroupID: group.ID, ProfileID: id}).Error)
	}
	return group
}

// seedTestProfile creates a standalone user_profiles row (no backing user —
// user_profiles.user_id is nullable) for use as an "other member" in tests.
func seedTestProfile(t *testing.T, name string) entity.UserProfile {
	t.Helper()
	profile := entity.UserProfile{Name: name}
	require.NoError(t, testDB.Create(&profile).Error)
	return profile
}
