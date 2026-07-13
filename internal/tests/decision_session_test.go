package tests

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sort"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yunobar/album/internal/domain/dto"
	"github.com/yunobar/album/internal/domain/entity"
	"github.com/yunobar/album/internal/testhelpers"
)

func TestDecisionSessionCreate(t *testing.T) {
	testhelpers.RequireTestDB(t, testDB)

	t.Run("creates a majority session with participants and candidates populated", func(t *testing.T) {
		testhelpers.TruncateAll(t, testDB)
		seedTestUser(t)
		bob := seedTestProfile(t, "Bob")
		group := seedTestGroup(t, nil, testProfileID, bob.ID)
		content1 := seedTestContent(t)
		content2 := seedTestContent(t)
		seedActiveWatchlistItem(t, testProfileID, content1.ID)
		seedActiveWatchlistItem(t, bob.ID, content2.ID)

		resp := postCreateSession(t, group.ID, dto.CreateSessionRequest{
			Method:              "majority",
			ParticipantIDs:      []uuid.UUID{testProfileID, bob.ID},
			CandidateContentIDs: []uuid.UUID{content1.ID, content2.ID},
		}, http.StatusCreated)

		assert.Equal(t, group.ID, resp.GroupID)
		assert.Equal(t, "majority", resp.Method)
		assert.Equal(t, "voting", resp.Status)
		assert.Nil(t, resp.CurrentChooserProfileID, "majority has no chooser concept")
		assert.Nil(t, resp.Tally)
		assert.Nil(t, resp.WinnerContentID)
		assert.Nil(t, resp.FinalizedAt)

		require.Len(t, resp.Participants, 2)
		assert.ElementsMatch(t, []uuid.UUID{testProfileID, bob.ID}, []uuid.UUID{resp.Participants[0].ID, resp.Participants[1].ID})

		require.Len(t, resp.Candidates, 2)
		assert.ElementsMatch(t, []uuid.UUID{content1.ID, content2.ID}, []uuid.UUID{resp.Candidates[0].ID, resp.Candidates[1].ID})

		// GET round-trips to the same shape.
		getResp := getSession(t, resp.ID, "", http.StatusOK)
		assert.Equal(t, resp.ID, getResp.ID)
		assert.Equal(t, resp.Method, getResp.Method)
		assert.Equal(t, resp.Status, getResp.Status)
		require.Len(t, getResp.Participants, 2)
		require.Len(t, getResp.Candidates, 2)
	})

	t.Run("400s when a participantId is not a member of the group", func(t *testing.T) {
		testhelpers.TruncateAll(t, testDB)
		seedTestUser(t)
		outsider := seedTestProfile(t, "Outsider")
		group := seedTestGroup(t, nil, testProfileID)
		content := seedTestContent(t)
		seedActiveWatchlistItem(t, testProfileID, content.ID)

		postCreateSession(t, group.ID, dto.CreateSessionRequest{
			Method:              "majority",
			ParticipantIDs:      []uuid.UUID{testProfileID, outsider.ID},
			CandidateContentIDs: []uuid.UUID{content.ID},
		}, http.StatusBadRequest)
	})

	t.Run("400s when a candidateContentId is not on the group's merged watchlist", func(t *testing.T) {
		testhelpers.TruncateAll(t, testDB)
		seedTestUser(t)
		group := seedTestGroup(t, nil, testProfileID)
		onWatchlist := seedTestContent(t)
		offWatchlist := seedTestContent(t)
		seedActiveWatchlistItem(t, testProfileID, onWatchlist.ID)

		postCreateSession(t, group.ID, dto.CreateSessionRequest{
			Method:              "majority",
			ParticipantIDs:      []uuid.UUID{testProfileID},
			CandidateContentIDs: []uuid.UUID{offWatchlist.ID},
		}, http.StatusBadRequest)
	})

	t.Run("404s when the caller is not a member of the group", func(t *testing.T) {
		testhelpers.TruncateAll(t, testDB)
		seedTestUser(t)
		owner := seedTestProfile(t, "Owner")
		group := seedTestGroup(t, nil, owner.ID)
		content := seedTestContent(t)
		seedActiveWatchlistItem(t, owner.ID, content.ID)

		postCreateSession(t, group.ID, dto.CreateSessionRequest{
			Method:              "majority",
			ParticipantIDs:      []uuid.UUID{owner.ID},
			CandidateContentIDs: []uuid.UUID{content.ID},
		}, http.StatusNotFound)
	})

	t.Run("priority method captures a snapshot per active watchlist match; other methods capture none", func(t *testing.T) {
		testhelpers.TruncateAll(t, testDB)
		seedTestUser(t)
		content := seedTestContent(t)
		group := seedTestGroup(t, nil, testProfileID)
		seedActiveWatchlistItem(t, testProfileID, content.ID)

		prioritySession := postCreateSession(t, group.ID, dto.CreateSessionRequest{
			Method:              "priority",
			ParticipantIDs:      []uuid.UUID{testProfileID},
			CandidateContentIDs: []uuid.UUID{content.ID},
		}, http.StatusCreated)

		var prioritySnapshots []entity.SessionPrioritySnapshot
		require.NoError(t, testDB.Where("session_id = ?", prioritySession.ID).Find(&prioritySnapshots).Error)
		assert.Len(t, prioritySnapshots, 1, "priority method must freeze one snapshot per on-watchlist candidate")

		majoritySession := postCreateSession(t, group.ID, dto.CreateSessionRequest{
			Method:              "majority",
			ParticipantIDs:      []uuid.UUID{testProfileID},
			CandidateContentIDs: []uuid.UUID{content.ID},
		}, http.StatusCreated)

		var majoritySnapshots []entity.SessionPrioritySnapshot
		require.NoError(t, testDB.Where("session_id = ?", majoritySession.ID).Find(&majoritySnapshots).Error)
		assert.Empty(t, majoritySnapshots, "non-priority methods must never capture snapshots")
	})

	t.Run("stores a non-zero random_seed and two sessions get different seeds", func(t *testing.T) {
		testhelpers.TruncateAll(t, testDB)
		seedTestUser(t)
		content := seedTestContent(t)
		group := seedTestGroup(t, nil, testProfileID)
		seedActiveWatchlistItem(t, testProfileID, content.ID)

		req := dto.CreateSessionRequest{
			Method:              "majority",
			ParticipantIDs:      []uuid.UUID{testProfileID},
			CandidateContentIDs: []uuid.UUID{content.ID},
		}
		s1 := postCreateSession(t, group.ID, req, http.StatusCreated)
		s2 := postCreateSession(t, group.ID, req, http.StatusCreated)

		var seed1, seed2 entity.DecisionSession
		require.NoError(t, testDB.First(&seed1, "id = ?", s1.ID).Error)
		require.NoError(t, testDB.First(&seed2, "id = ?", s2.ID).Error)

		assert.NotZero(t, seed1.RandomSeed)
		assert.NotZero(t, seed2.RandomSeed)
		assert.NotEqual(t, seed1.RandomSeed, seed2.RandomSeed, "a weak but real check the generator isn't a constant")
	})

	t.Run("roundRobin exposes the server-decided chooser at the group's round_robin_pointer", func(t *testing.T) {
		testhelpers.TruncateAll(t, testDB)
		seedTestUser(t)
		bob := seedTestProfile(t, "Bob")
		carol := seedTestProfile(t, "Carol")
		group := seedTestGroup(t, nil, testProfileID, bob.ID, carol.ID)
		content := seedTestContent(t)
		seedActiveWatchlistItem(t, testProfileID, content.ID)

		var members []entity.GroupMember
		require.NoError(t, testDB.Where("group_id = ?", group.ID).Find(&members).Error)
		sort.Slice(members, func(i, j int) bool { return members[i].ID.String() < members[j].ID.String() })
		require.Len(t, members, 3)

		created := postCreateSession(t, group.ID, dto.CreateSessionRequest{
			Method:              "roundRobin",
			ParticipantIDs:      []uuid.UUID{testProfileID, bob.ID, carol.ID},
			CandidateContentIDs: []uuid.UUID{content.ID},
		}, http.StatusCreated)
		assert.Equal(t, "roundRobin", created.Method)
		require.NotNil(t, created.CurrentChooserProfileID)
		assert.Equal(t, members[0].ProfileID, *created.CurrentChooserProfileID, "pointer defaults to 0")

		require.NoError(t, testDB.Model(&entity.Group{}).Where("id = ?", group.ID).Update("round_robin_pointer", 2).Error)

		fetched := getSession(t, created.ID, "", http.StatusOK)
		require.NotNil(t, fetched.CurrentChooserProfileID)
		assert.Equal(t, members[2].ProfileID, *fetched.CurrentChooserProfileID)
	})
}

func TestDecisionSessionGet(t *testing.T) {
	testhelpers.RequireTestDB(t, testDB)

	t.Run("404s for a group member who is not a session participant", func(t *testing.T) {
		testhelpers.TruncateAll(t, testDB)
		seedTestUser(t)
		bob := seedTestProfile(t, "Bob")
		group := seedTestGroup(t, nil, testProfileID, bob.ID)
		content := seedTestContent(t)
		seedActiveWatchlistItem(t, testProfileID, content.ID)

		created := postCreateSession(t, group.ID, dto.CreateSessionRequest{
			Method:              "majority",
			ParticipantIDs:      []uuid.UUID{testProfileID},
			CandidateContentIDs: []uuid.UUID{content.ID},
		}, http.StatusCreated)

		getSession(t, created.ID, bob.ID.String(), http.StatusNotFound)
	})

	t.Run("404s for a session ID that doesn't exist", func(t *testing.T) {
		testhelpers.TruncateAll(t, testDB)
		seedTestUser(t)

		getSession(t, uuid.New(), "", http.StatusNotFound)
	})
}

// postCreateSession POSTs a create-session request for groupID as the
// default test caller and asserts wantStatus. On a 2xx status it decodes and
// returns the response body's data.
func postCreateSession(t *testing.T, groupID uuid.UUID, req dto.CreateSessionRequest, wantStatus int) dto.SessionResponse {
	t.Helper()

	body, err := json.Marshal(req)
	require.NoError(t, err)

	w := httptest.NewRecorder()
	httpReq, _ := http.NewRequest(http.MethodPost, "/api/v1/groups/"+groupID.String()+"/sessions", bytes.NewReader(body))
	httpReq.Header.Set("Content-Type", "application/json")
	testRouter.ServeHTTP(w, httpReq)
	require.Equal(t, wantStatus, w.Code, w.Body.String())

	var resp struct {
		Data dto.SessionResponse `json:"data"`
	}
	if w.Code < 300 {
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	}
	return resp.Data
}

// getSession GETs a session's detail. callerProfileID overrides the default
// test caller via testProfileIDHeader when non-empty.
func getSession(t *testing.T, sessionID uuid.UUID, callerProfileID string, wantStatus int) dto.SessionResponse {
	t.Helper()

	w := httptest.NewRecorder()
	httpReq, _ := http.NewRequest(http.MethodGet, "/api/v1/sessions/"+sessionID.String(), nil)
	if callerProfileID != "" {
		httpReq.Header.Set(testProfileIDHeader, callerProfileID)
	}
	testRouter.ServeHTTP(w, httpReq)
	require.Equal(t, wantStatus, w.Code, w.Body.String())

	var resp struct {
		Data dto.SessionResponse `json:"data"`
	}
	if w.Code < 300 {
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	}
	return resp.Data
}

func seedActiveWatchlistItem(t *testing.T, profileID, contentID uuid.UUID) {
	t.Helper()
	require.NoError(t, testDB.Create(&entity.WatchlistItem{
		ProfileID: profileID,
		ContentID: contentID,
		Priority:  "medium",
		Status:    "active",
	}).Error)
}
