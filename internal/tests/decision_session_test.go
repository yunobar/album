package tests

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yunobar/album/internal/core/pubsub"
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
		// Majority now has a live tally (Task 3) — zero votes cast yet, so
		// it's a CountsTally with an empty map, not null.
		assert.Equal(t, map[string]any{"counts": map[string]any{}}, resp.Tally)
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

	t.Run("deduplicates repeated participantIds and candidateContentIds instead of erroring", func(t *testing.T) {
		testhelpers.TruncateAll(t, testDB)
		seedTestUser(t)
		bob := seedTestProfile(t, "Bob")
		group := seedTestGroup(t, nil, testProfileID, bob.ID)
		content := seedTestContent(t)
		seedActiveWatchlistItem(t, testProfileID, content.ID)

		resp := postCreateSession(t, group.ID, dto.CreateSessionRequest{
			Method:              "majority",
			ParticipantIDs:      []uuid.UUID{testProfileID, testProfileID, bob.ID},
			CandidateContentIDs: []uuid.UUID{content.ID, content.ID},
		}, http.StatusCreated)

		require.Len(t, resp.Participants, 2)
		require.Len(t, resp.Candidates, 1)
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

	t.Run("400s when the caller (a group member) excludes themselves from participantIds", func(t *testing.T) {
		testhelpers.TruncateAll(t, testDB)
		seedTestUser(t)
		bob := seedTestProfile(t, "Bob")
		group := seedTestGroup(t, nil, testProfileID, bob.ID)
		content := seedTestContent(t)
		seedActiveWatchlistItem(t, bob.ID, content.ID)

		// The caller is a genuine group member (unlike the 404 case above),
		// so without this validation the create would commit and this same
		// request's own follow-up Get reload would then 404 the caller on
		// the session they just made.
		postCreateSession(t, group.ID, dto.CreateSessionRequest{
			Method:              "majority",
			ParticipantIDs:      []uuid.UUID{bob.ID},
			CandidateContentIDs: []uuid.UUID{content.ID},
		}, http.StatusBadRequest)
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

// countsTallyEnvelope decodes just the shape TallyResponse/SessionResponse
// take when method is majority/ranked — {"tally":{"counts":{...}}} — without
// resorting to `any` type assertions in every test.
type countsTallyEnvelope struct {
	Tally struct {
		Counts map[string]int `json:"counts"`
	} `json:"tally"`
}

// selectionTallyEnvelope is the round_robin equivalent —
// {"tally":{"selectedContentId":"..."}}.
type selectionTallyEnvelope struct {
	Tally struct {
		SelectedContentID *uuid.UUID `json:"selectedContentId"`
	} `json:"tally"`
}

// postSessionAction POSTs body to /api/v1/sessions/{sessionID}/{action} as
// callerProfileID (default test caller when empty) and asserts wantStatus.
// Returns the raw response body for the caller to decode per-endpoint.
func postSessionAction(t *testing.T, sessionID uuid.UUID, action string, body any, callerProfileID string, wantStatus int) []byte {
	t.Helper()

	b, err := json.Marshal(body)
	require.NoError(t, err)

	w := httptest.NewRecorder()
	httpReq, _ := http.NewRequest(http.MethodPost, "/api/v1/sessions/"+sessionID.String()+"/"+action, bytes.NewReader(b))
	httpReq.Header.Set("Content-Type", "application/json")
	if callerProfileID != "" {
		httpReq.Header.Set(testProfileIDHeader, callerProfileID)
	}
	testRouter.ServeHTTP(w, httpReq)
	require.Equal(t, wantStatus, w.Code, w.Body.String())

	return w.Body.Bytes()
}

func TestDecisionSessionCastVote(t *testing.T) {
	testhelpers.RequireTestDB(t, testDB)

	t.Run("round-trips votes, replacing rather than stacking a resubmit", func(t *testing.T) {
		testhelpers.TruncateAll(t, testDB)
		seedTestUser(t)
		bob := seedTestProfile(t, "Bob")
		group := seedTestGroup(t, nil, testProfileID, bob.ID)
		content1 := seedTestContent(t)
		content2 := seedTestContent(t)
		seedActiveWatchlistItem(t, testProfileID, content1.ID)
		seedActiveWatchlistItem(t, bob.ID, content2.ID)

		session := postCreateSession(t, group.ID, dto.CreateSessionRequest{
			Method:              "majority",
			ParticipantIDs:      []uuid.UUID{testProfileID, bob.ID},
			CandidateContentIDs: []uuid.UUID{content1.ID, content2.ID},
		}, http.StatusCreated)

		// Each response is decoded into a fresh envelope — json.Unmarshal
		// merges into an existing map rather than clearing it first, so
		// reusing one across calls would leak stale keys from an earlier
		// response into a later assertion.
		body := postSessionAction(t, session.ID, "votes", dto.CastVoteRequest{ContentID: content1.ID}, "", http.StatusOK)
		var env1 countsTallyEnvelope
		require.NoError(t, json.Unmarshal(body, mustDataEnvelope(&env1)))
		assert.Equal(t, 1, env1.Tally.Counts[content1.ID.String()])

		body = postSessionAction(t, session.ID, "votes", dto.CastVoteRequest{ContentID: content1.ID}, bob.ID.String(), http.StatusOK)
		var env2 countsTallyEnvelope
		require.NoError(t, json.Unmarshal(body, mustDataEnvelope(&env2)))
		assert.Equal(t, 2, env2.Tally.Counts[content1.ID.String()])

		// testProfileID changes their mind — replace, not stack.
		body = postSessionAction(t, session.ID, "votes", dto.CastVoteRequest{ContentID: content2.ID}, "", http.StatusOK)
		var env3 countsTallyEnvelope
		require.NoError(t, json.Unmarshal(body, mustDataEnvelope(&env3)))
		assert.Equal(t, 1, env3.Tally.Counts[content1.ID.String()])
		assert.Equal(t, 1, env3.Tally.Counts[content2.ID.String()])

		getBody, err := json.Marshal(getSession(t, session.ID, "", http.StatusOK))
		require.NoError(t, err)
		var getEnv countsTallyEnvelope
		require.NoError(t, json.Unmarshal(getBody, &getEnv))
		assert.Equal(t, 1, getEnv.Tally.Counts[content1.ID.String()])
		assert.Equal(t, 1, getEnv.Tally.Counts[content2.ID.String()])
	})

	t.Run("400s on a non-candidate contentId", func(t *testing.T) {
		testhelpers.TruncateAll(t, testDB)
		seedTestUser(t)
		group := seedTestGroup(t, nil, testProfileID)
		content := seedTestContent(t)
		offCandidate := seedTestContent(t)
		seedActiveWatchlistItem(t, testProfileID, content.ID)

		session := postCreateSession(t, group.ID, dto.CreateSessionRequest{
			Method:              "majority",
			ParticipantIDs:      []uuid.UUID{testProfileID},
			CandidateContentIDs: []uuid.UUID{content.ID},
		}, http.StatusCreated)

		postSessionAction(t, session.ID, "votes", dto.CastVoteRequest{ContentID: offCandidate.ID}, "", http.StatusBadRequest)
	})

	t.Run("400s when posting to /votes on a ranked-method session", func(t *testing.T) {
		testhelpers.TruncateAll(t, testDB)
		seedTestUser(t)
		group := seedTestGroup(t, nil, testProfileID)
		content := seedTestContent(t)
		seedActiveWatchlistItem(t, testProfileID, content.ID)

		session := postCreateSession(t, group.ID, dto.CreateSessionRequest{
			Method:              "ranked",
			ParticipantIDs:      []uuid.UUID{testProfileID},
			CandidateContentIDs: []uuid.UUID{content.ID},
		}, http.StatusCreated)

		postSessionAction(t, session.ID, "votes", dto.CastVoteRequest{ContentID: content.ID}, "", http.StatusBadRequest)
	})

	t.Run("404s for a group member who is not a session participant", func(t *testing.T) {
		testhelpers.TruncateAll(t, testDB)
		seedTestUser(t)
		bob := seedTestProfile(t, "Bob")
		group := seedTestGroup(t, nil, testProfileID, bob.ID)
		content := seedTestContent(t)
		seedActiveWatchlistItem(t, testProfileID, content.ID)

		session := postCreateSession(t, group.ID, dto.CreateSessionRequest{
			Method:              "majority",
			ParticipantIDs:      []uuid.UUID{testProfileID},
			CandidateContentIDs: []uuid.UUID{content.ID},
		}, http.StatusCreated)

		postSessionAction(t, session.ID, "votes", dto.CastVoteRequest{ContentID: content.ID}, bob.ID.String(), http.StatusNotFound)
	})

	t.Run("409s once the session is no longer open for voting", func(t *testing.T) {
		testhelpers.TruncateAll(t, testDB)
		seedTestUser(t)
		group := seedTestGroup(t, nil, testProfileID)
		content := seedTestContent(t)
		seedActiveWatchlistItem(t, testProfileID, content.ID)

		session := postCreateSession(t, group.ID, dto.CreateSessionRequest{
			Method:              "majority",
			ParticipantIDs:      []uuid.UUID{testProfileID},
			CandidateContentIDs: []uuid.UUID{content.ID},
		}, http.StatusCreated)

		require.NoError(t, testDB.Model(&entity.DecisionSession{}).Where("id = ?", session.ID).Update("status", "completed").Error)

		postSessionAction(t, session.ID, "votes", dto.CastVoteRequest{ContentID: content.ID}, "", http.StatusConflict)
	})
}

func TestDecisionSessionSubmitRanking(t *testing.T) {
	testhelpers.RequireTestDB(t, testDB)

	t.Run("round-trips a ranked ballot, replacing rather than accumulating on resubmit", func(t *testing.T) {
		testhelpers.TruncateAll(t, testDB)
		seedTestUser(t)
		bob := seedTestProfile(t, "Bob")
		group := seedTestGroup(t, nil, testProfileID, bob.ID)
		content1 := seedTestContent(t)
		content2 := seedTestContent(t)
		seedActiveWatchlistItem(t, testProfileID, content1.ID)
		seedActiveWatchlistItem(t, bob.ID, content2.ID)

		session := postCreateSession(t, group.ID, dto.CreateSessionRequest{
			Method:              "ranked",
			ParticipantIDs:      []uuid.UUID{testProfileID, bob.ID},
			CandidateContentIDs: []uuid.UUID{content1.ID, content2.ID},
		}, http.StatusCreated)

		// testProfileID ranks content2 first, bob ranks content1 first.
		// Each response is decoded into a fresh envelope — see the /votes
		// test above for why reusing one across calls is unsafe.
		postSessionAction(t, session.ID, "rankings", dto.SubmitRankingRequest{Ranking: []uuid.UUID{content2.ID, content1.ID}}, "", http.StatusOK)
		body := postSessionAction(t, session.ID, "rankings", dto.SubmitRankingRequest{Ranking: []uuid.UUID{content1.ID, content2.ID}}, bob.ID.String(), http.StatusOK)

		var env1 countsTallyEnvelope
		require.NoError(t, json.Unmarshal(body, mustDataEnvelope(&env1)))
		assert.Equal(t, 1, env1.Tally.Counts[content1.ID.String()])
		assert.Equal(t, 1, env1.Tally.Counts[content2.ID.String()])

		// testProfileID resubmits, now ranking content1 first — replaces
		// their prior ballot rather than adding a second one.
		body = postSessionAction(t, session.ID, "rankings", dto.SubmitRankingRequest{Ranking: []uuid.UUID{content1.ID, content2.ID}}, "", http.StatusOK)
		var env2 countsTallyEnvelope
		require.NoError(t, json.Unmarshal(body, mustDataEnvelope(&env2)))
		assert.Equal(t, 2, env2.Tally.Counts[content1.ID.String()])
		assert.Equal(t, 0, env2.Tally.Counts[content2.ID.String()])

		getBody, err := json.Marshal(getSession(t, session.ID, "", http.StatusOK))
		require.NoError(t, err)
		var getEnv countsTallyEnvelope
		require.NoError(t, json.Unmarshal(getBody, &getEnv))
		assert.Equal(t, 2, getEnv.Tally.Counts[content1.ID.String()])
	})

	t.Run("400s on a duplicate candidate in the ranking", func(t *testing.T) {
		testhelpers.TruncateAll(t, testDB)
		seedTestUser(t)
		group := seedTestGroup(t, nil, testProfileID)
		content := seedTestContent(t)
		seedActiveWatchlistItem(t, testProfileID, content.ID)

		session := postCreateSession(t, group.ID, dto.CreateSessionRequest{
			Method:              "ranked",
			ParticipantIDs:      []uuid.UUID{testProfileID},
			CandidateContentIDs: []uuid.UUID{content.ID},
		}, http.StatusCreated)

		postSessionAction(t, session.ID, "rankings", dto.SubmitRankingRequest{Ranking: []uuid.UUID{content.ID, content.ID}}, "", http.StatusBadRequest)
	})

	t.Run("400s on a non-candidate entry", func(t *testing.T) {
		testhelpers.TruncateAll(t, testDB)
		seedTestUser(t)
		group := seedTestGroup(t, nil, testProfileID)
		content := seedTestContent(t)
		offCandidate := seedTestContent(t)
		seedActiveWatchlistItem(t, testProfileID, content.ID)

		session := postCreateSession(t, group.ID, dto.CreateSessionRequest{
			Method:              "ranked",
			ParticipantIDs:      []uuid.UUID{testProfileID},
			CandidateContentIDs: []uuid.UUID{content.ID},
		}, http.StatusCreated)

		postSessionAction(t, session.ID, "rankings", dto.SubmitRankingRequest{Ranking: []uuid.UUID{content.ID, offCandidate.ID}}, "", http.StatusBadRequest)
	})
}

func TestDecisionSessionSelect(t *testing.T) {
	testhelpers.RequireTestDB(t, testDB)

	t.Run("chooser's pick round-trips through GET; a non-chooser is forbidden", func(t *testing.T) {
		testhelpers.TruncateAll(t, testDB)
		seedTestUser(t)
		bob := seedTestProfile(t, "Bob")
		group := seedTestGroup(t, nil, testProfileID, bob.ID)
		content := seedTestContent(t)
		seedActiveWatchlistItem(t, testProfileID, content.ID)

		session := postCreateSession(t, group.ID, dto.CreateSessionRequest{
			Method:              "roundRobin",
			ParticipantIDs:      []uuid.UUID{testProfileID, bob.ID},
			CandidateContentIDs: []uuid.UUID{content.ID},
		}, http.StatusCreated)
		require.NotNil(t, session.CurrentChooserProfileID)

		// nonChooser/chooserHeader: "" means the default test caller
		// (testProfileID). Whichever of the two participants the server
		// picked as chooser, the other one is the non-chooser.
		nonChooser, chooserHeader := bob.ID, ""
		if *session.CurrentChooserProfileID != testProfileID {
			nonChooser, chooserHeader = testProfileID, session.CurrentChooserProfileID.String()
		}
		postSessionAction(t, session.ID, "select", dto.CastVoteRequest{ContentID: content.ID}, nonChooser.String(), http.StatusForbidden)

		body := postSessionAction(t, session.ID, "select", dto.CastVoteRequest{ContentID: content.ID}, chooserHeader, http.StatusOK)

		var env selectionTallyEnvelope
		require.NoError(t, json.Unmarshal(body, mustDataEnvelope(&env)))
		require.NotNil(t, env.Tally.SelectedContentID)
		assert.Equal(t, content.ID, *env.Tally.SelectedContentID)

		getBody, err := json.Marshal(getSession(t, session.ID, "", http.StatusOK))
		require.NoError(t, err)
		var getEnv selectionTallyEnvelope
		require.NoError(t, json.Unmarshal(getBody, &getEnv))
		require.NotNil(t, getEnv.Tally.SelectedContentID)
		assert.Equal(t, content.ID, *getEnv.Tally.SelectedContentID)
	})

	t.Run("400s when posting to /select on a random-method session", func(t *testing.T) {
		testhelpers.TruncateAll(t, testDB)
		seedTestUser(t)
		group := seedTestGroup(t, nil, testProfileID)
		content := seedTestContent(t)
		seedActiveWatchlistItem(t, testProfileID, content.ID)

		session := postCreateSession(t, group.ID, dto.CreateSessionRequest{
			Method:              "random",
			ParticipantIDs:      []uuid.UUID{testProfileID},
			CandidateContentIDs: []uuid.UUID{content.ID},
		}, http.StatusCreated)

		postSessionAction(t, session.ID, "select", dto.CastVoteRequest{ContentID: content.ID}, "", http.StatusBadRequest)
	})
}

// postFinalize POSTs /finalize for sessionID as callerProfileID (default
// test caller when empty) and asserts wantStatus. On a 2xx status it decodes
// and returns the response body's data.
func postFinalize(t *testing.T, sessionID uuid.UUID, callerProfileID string, wantStatus int) dto.SessionResponse {
	t.Helper()

	w := httptest.NewRecorder()
	httpReq, _ := http.NewRequest(http.MethodPost, "/api/v1/sessions/"+sessionID.String()+"/finalize", nil)
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

// seedWatchlistItemWithPriority is seedActiveWatchlistItem with an explicit
// priority, for tests that need to distinguish Priority-Based weights.
func seedWatchlistItemWithPriority(t *testing.T, profileID, contentID uuid.UUID, priority string) {
	t.Helper()
	require.NoError(t, testDB.Create(&entity.WatchlistItem{
		ProfileID: profileID,
		ContentID: contentID,
		Priority:  priority,
		Status:    "active",
	}).Error)
}

func TestDecisionSessionFinalize(t *testing.T) {
	testhelpers.RequireTestDB(t, testDB)

	t.Run("immutability: finalize locks in the winner; a later mutation 409s and the winner is unchanged", func(t *testing.T) {
		testhelpers.TruncateAll(t, testDB)
		seedTestUser(t)
		bob := seedTestProfile(t, "Bob")
		group := seedTestGroup(t, nil, testProfileID, bob.ID)
		content1 := seedTestContent(t)
		content2 := seedTestContent(t)
		seedActiveWatchlistItem(t, testProfileID, content1.ID)
		seedActiveWatchlistItem(t, bob.ID, content2.ID)

		session := postCreateSession(t, group.ID, dto.CreateSessionRequest{
			Method:              "majority",
			ParticipantIDs:      []uuid.UUID{testProfileID, bob.ID},
			CandidateContentIDs: []uuid.UUID{content1.ID, content2.ID},
		}, http.StatusCreated)

		// Two participants vote for different candidates so the outcome
		// isn't trivially predetermined by a single vote.
		postSessionAction(t, session.ID, "votes", dto.CastVoteRequest{ContentID: content1.ID}, "", http.StatusOK)
		postSessionAction(t, session.ID, "votes", dto.CastVoteRequest{ContentID: content2.ID}, bob.ID.String(), http.StatusOK)
		postSessionAction(t, session.ID, "votes", dto.CastVoteRequest{ContentID: content1.ID}, bob.ID.String(), http.StatusOK)

		finalized := postFinalize(t, session.ID, "", http.StatusOK)
		assert.Equal(t, "completed", finalized.Status)
		require.NotNil(t, finalized.WinnerContentID)
		assert.Equal(t, content1.ID, *finalized.WinnerContentID)
		require.NotNil(t, finalized.FinalizedAt)

		// The Participants/Candidates has-many association rows must
		// survive Update's db.Save call untouched — same count, same IDs —
		// this is the GORM Update-with-associations invariant the brief
		// asked to be verified against real data, not just read and
		// trusted.
		require.Len(t, finalized.Participants, 2)
		require.Len(t, finalized.Candidates, 2)

		// A follow-up mutation is rejected — the session is no longer open
		// for voting.
		postSessionAction(t, session.ID, "votes", dto.CastVoteRequest{ContentID: content2.ID}, "", http.StatusConflict)

		// The winner is unchanged.
		getResp := getSession(t, session.ID, "", http.StatusOK)
		require.NotNil(t, getResp.WinnerContentID)
		assert.Equal(t, content1.ID, *getResp.WinnerContentID)
		assert.Equal(t, "completed", getResp.Status)
		require.Len(t, getResp.Participants, 2)
		require.Len(t, getResp.Candidates, 2)

		// Belt-and-suspenders: confirm the association rows in the DB
		// weren't duplicated or orphaned by Update's db.Save call on the
		// preloaded session.
		var participantCount, candidateCount int64
		require.NoError(t, testDB.Model(&entity.SessionParticipant{}).Where("session_id = ?", session.ID).Count(&participantCount).Error)
		require.NoError(t, testDB.Model(&entity.SessionCandidate{}).Where("session_id = ?", session.ID).Count(&candidateCount).Error)
		assert.EqualValues(t, 2, participantCount)
		assert.EqualValues(t, 2, candidateCount)
	})

	t.Run("snapshot: finalize uses the priority frozen at session creation, not a later edit", func(t *testing.T) {
		testhelpers.TruncateAll(t, testDB)
		seedTestUser(t)
		bob := seedTestProfile(t, "Bob")
		group := seedTestGroup(t, nil, testProfileID, bob.ID)
		candidateA := seedTestContent(t)
		candidateB := seedTestContent(t)

		// testProfileID's priority for candidateA starts "low" (weight 1);
		// bob's priority for candidateB is "medium" (weight 2) — B wins
		// under the frozen snapshot. If the resolver read live
		// watchlist_items instead of the snapshot, editing A to "must"
		// (weight 5) after creation would flip the winner to A.
		seedWatchlistItemWithPriority(t, testProfileID, candidateA.ID, "low")
		seedWatchlistItemWithPriority(t, bob.ID, candidateB.ID, "medium")

		session := postCreateSession(t, group.ID, dto.CreateSessionRequest{
			Method:              "priority",
			ParticipantIDs:      []uuid.UUID{testProfileID, bob.ID},
			CandidateContentIDs: []uuid.UUID{candidateA.ID, candidateB.ID},
		}, http.StatusCreated)

		// Bypass the watchlist endpoint entirely — edit the underlying row
		// directly, after the session (and its frozen snapshot) already
		// exist.
		require.NoError(t, testDB.Model(&entity.WatchlistItem{}).
			Where("profile_id = ? AND content_id = ?", testProfileID, candidateA.ID).
			Update("priority", "must").Error)

		finalized := postFinalize(t, session.ID, "", http.StatusOK)

		require.NotNil(t, finalized.WinnerContentID)
		assert.Equal(t, candidateB.ID, *finalized.WinnerContentID, "must use the frozen 'low' snapshot, not the edited 'must' value")
	})

	t.Run("ranked: round-trips to a winner from the candidate set", func(t *testing.T) {
		testhelpers.TruncateAll(t, testDB)
		seedTestUser(t)
		bob := seedTestProfile(t, "Bob")
		group := seedTestGroup(t, nil, testProfileID, bob.ID)
		content1 := seedTestContent(t)
		content2 := seedTestContent(t)
		seedActiveWatchlistItem(t, testProfileID, content1.ID)
		seedActiveWatchlistItem(t, bob.ID, content2.ID)

		session := postCreateSession(t, group.ID, dto.CreateSessionRequest{
			Method:              "ranked",
			ParticipantIDs:      []uuid.UUID{testProfileID, bob.ID},
			CandidateContentIDs: []uuid.UUID{content1.ID, content2.ID},
		}, http.StatusCreated)

		postSessionAction(t, session.ID, "rankings", dto.SubmitRankingRequest{Ranking: []uuid.UUID{content1.ID, content2.ID}}, "", http.StatusOK)
		postSessionAction(t, session.ID, "rankings", dto.SubmitRankingRequest{Ranking: []uuid.UUID{content2.ID, content1.ID}}, bob.ID.String(), http.StatusOK)

		finalized := postFinalize(t, session.ID, "", http.StatusOK)

		assert.Equal(t, "completed", finalized.Status)
		require.NotNil(t, finalized.WinnerContentID)
		assert.Contains(t, []uuid.UUID{content1.ID, content2.ID}, *finalized.WinnerContentID)
	})

	t.Run("round_robin: winner matches the chooser's pick and the group's round_robin_pointer advances", func(t *testing.T) {
		testhelpers.TruncateAll(t, testDB)
		seedTestUser(t)
		bob := seedTestProfile(t, "Bob")
		group := seedTestGroup(t, nil, testProfileID, bob.ID)
		content := seedTestContent(t)
		seedActiveWatchlistItem(t, testProfileID, content.ID)

		session := postCreateSession(t, group.ID, dto.CreateSessionRequest{
			Method:              "roundRobin",
			ParticipantIDs:      []uuid.UUID{testProfileID, bob.ID},
			CandidateContentIDs: []uuid.UUID{content.ID},
		}, http.StatusCreated)
		require.NotNil(t, session.CurrentChooserProfileID)

		chooserHeader := ""
		if *session.CurrentChooserProfileID != testProfileID {
			chooserHeader = session.CurrentChooserProfileID.String()
		}
		postSessionAction(t, session.ID, "select", dto.CastVoteRequest{ContentID: content.ID}, chooserHeader, http.StatusOK)

		var groupBefore entity.Group
		require.NoError(t, testDB.First(&groupBefore, "id = ?", group.ID).Error)

		finalized := postFinalize(t, session.ID, "", http.StatusOK)

		assert.Equal(t, "completed", finalized.Status)
		require.NotNil(t, finalized.WinnerContentID)
		assert.Equal(t, content.ID, *finalized.WinnerContentID)

		var groupAfter entity.Group
		require.NoError(t, testDB.First(&groupAfter, "id = ?", group.ID).Error)
		assert.Equal(t, groupBefore.RoundRobinPointer+1, groupAfter.RoundRobinPointer)
	})

	t.Run("random: round-trips to a winner from the candidate set with no live input", func(t *testing.T) {
		testhelpers.TruncateAll(t, testDB)
		seedTestUser(t)
		group := seedTestGroup(t, nil, testProfileID)
		content1 := seedTestContent(t)
		content2 := seedTestContent(t)
		seedActiveWatchlistItem(t, testProfileID, content1.ID)
		seedActiveWatchlistItem(t, testProfileID, content2.ID)

		session := postCreateSession(t, group.ID, dto.CreateSessionRequest{
			Method:              "random",
			ParticipantIDs:      []uuid.UUID{testProfileID},
			CandidateContentIDs: []uuid.UUID{content1.ID, content2.ID},
		}, http.StatusCreated)

		finalized := postFinalize(t, session.ID, "", http.StatusOK)

		assert.Equal(t, "completed", finalized.Status)
		require.NotNil(t, finalized.WinnerContentID)
		assert.Contains(t, []uuid.UUID{content1.ID, content2.ID}, *finalized.WinnerContentID)
	})

	t.Run("404s for a group member who is not a session participant", func(t *testing.T) {
		testhelpers.TruncateAll(t, testDB)
		seedTestUser(t)
		bob := seedTestProfile(t, "Bob")
		group := seedTestGroup(t, nil, testProfileID, bob.ID)
		content := seedTestContent(t)
		seedActiveWatchlistItem(t, testProfileID, content.ID)

		session := postCreateSession(t, group.ID, dto.CreateSessionRequest{
			Method:              "majority",
			ParticipantIDs:      []uuid.UUID{testProfileID},
			CandidateContentIDs: []uuid.UUID{content.ID},
		}, http.StatusCreated)

		postFinalize(t, session.ID, bob.ID.String(), http.StatusNotFound)
	})

	t.Run("409s when finalizing an already-completed session", func(t *testing.T) {
		testhelpers.TruncateAll(t, testDB)
		seedTestUser(t)
		group := seedTestGroup(t, nil, testProfileID)
		content := seedTestContent(t)
		seedActiveWatchlistItem(t, testProfileID, content.ID)

		session := postCreateSession(t, group.ID, dto.CreateSessionRequest{
			Method:              "random",
			ParticipantIDs:      []uuid.UUID{testProfileID},
			CandidateContentIDs: []uuid.UUID{content.ID},
		}, http.StatusCreated)

		postFinalize(t, session.ID, "", http.StatusOK)
		postFinalize(t, session.ID, "", http.StatusConflict)
	})

	// Regression test for the TOCTOU double-finalize race: a sequential test
	// can't catch it because it can't force two Finalize calls to overlap
	// their read-then-write. This fires them concurrently at the real DB and
	// relies on ForUpdate row locking to serialize them, asserting exactly
	// one 200 and one 409 — never two 200s (which would mean one finalize
	// silently overwrote the other's winner).
	t.Run("concurrency: two simultaneous finalize calls on the same session yield exactly one winner", func(t *testing.T) {
		testhelpers.TruncateAll(t, testDB)
		seedTestUser(t)
		group := seedTestGroup(t, nil, testProfileID)
		content := seedTestContent(t)
		seedActiveWatchlistItem(t, testProfileID, content.ID)

		session := postCreateSession(t, group.ID, dto.CreateSessionRequest{
			Method:              "random",
			ParticipantIDs:      []uuid.UUID{testProfileID},
			CandidateContentIDs: []uuid.UUID{content.ID},
		}, http.StatusCreated)

		const attempts = 10
		codes := make([]int, attempts)
		var wg sync.WaitGroup
		for i := range attempts {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				w := httptest.NewRecorder()
				req, _ := http.NewRequest(http.MethodPost, "/api/v1/sessions/"+session.ID.String()+"/finalize", nil)
				testRouter.ServeHTTP(w, req)
				codes[i] = w.Code
			}(i)
		}
		wg.Wait()

		var okCount, conflictCount int
		for _, code := range codes {
			switch code {
			case http.StatusOK:
				okCount++
			case http.StatusConflict:
				conflictCount++
			}
		}
		assert.Equal(t, 1, okCount, "exactly one concurrent finalize call should win with 200")
		assert.Equal(t, attempts-1, conflictCount, "every other concurrent call should 409, not silently re-resolve")
	})
}

// waitForLiveSubscription blocks until conn has genuinely received a
// message published on sessionID's live subject, proving the server's
// ChanSubscribe call has completed — a WS Dial() only confirms the HTTP 101
// upgrade, which happens earlier in the handler than the subscribe call.
// Publishes canaries on a short interval via the real NATS connection until
// one round-trips back, instead of guessing a fixed sleep duration.
func waitForLiveSubscription(t *testing.T, conn *websocket.Conn, sessionID uuid.UUID) {
	t.Helper()

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		require.NoError(t, testNATS.Publish(pubsub.LiveSubject(sessionID), []byte(`{"type":"ready"}`)))

		require.NoError(t, conn.SetReadDeadline(time.Now().Add(100*time.Millisecond)))
		_, data, err := conn.ReadMessage()
		if err != nil {
			continue // read timeout — subscription (or the canary) hasn't landed yet
		}

		var probe struct {
			Type string `json:"type"`
		}
		if json.Unmarshal(data, &probe) == nil && probe.Type == "ready" {
			return
		}
	}
	t.Fatal("timed out waiting for the WS handler's NATS subscription to become active")
}

func TestDecisionSessionLive(t *testing.T) {
	testhelpers.RequireTestDB(t, testDB)

	// Not NATS-gated: a non-participant is rejected by VerifyParticipant
	// before the WS upgrade even happens, so this is a plain HTTP
	// assertion — no WS client needed, and it runs everywhere (including
	// without a local NATS server).
	t.Run("404s for a non-participant without upgrading the connection", func(t *testing.T) {
		testhelpers.TruncateAll(t, testDB)
		seedTestUser(t)
		bob := seedTestProfile(t, "Bob")
		group := seedTestGroup(t, nil, testProfileID)
		content := seedTestContent(t)
		seedActiveWatchlistItem(t, testProfileID, content.ID)

		session := postCreateSession(t, group.ID, dto.CreateSessionRequest{
			Method:              "majority",
			ParticipantIDs:      []uuid.UUID{testProfileID},
			CandidateContentIDs: []uuid.UUID{content.ID},
		}, http.StatusCreated)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/api/v1/sessions/"+session.ID.String()+"/live", nil)
		req.Header.Set(testProfileIDHeader, bob.ID.String())
		testRouter.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	// Not NATS-gated: a rejected Origin fails the WS upgrade itself, before
	// the handler ever reaches ChanSubscribe.
	t.Run("rejects the WS upgrade when Origin is missing or not allowlisted", func(t *testing.T) {
		testhelpers.TruncateAll(t, testDB)
		seedTestUser(t)
		group := seedTestGroup(t, nil, testProfileID)
		content := seedTestContent(t)
		seedActiveWatchlistItem(t, testProfileID, content.ID)

		session := postCreateSession(t, group.ID, dto.CreateSessionRequest{
			Method:              "majority",
			ParticipantIDs:      []uuid.UUID{testProfileID},
			CandidateContentIDs: []uuid.UUID{content.ID},
		}, http.StatusCreated)

		httpServer := httptest.NewServer(testRouter)
		defer httpServer.Close()

		wsURL := "ws" + strings.TrimPrefix(httpServer.URL, "http") + "/api/v1/sessions/" + session.ID.String() + "/live"

		t.Run("no Origin header", func(t *testing.T) {
			header := http.Header{}
			header.Set(testProfileIDHeader, testProfileID.String())
			conn, resp, err := websocket.DefaultDialer.Dial(wsURL, header)
			require.Error(t, err)
			if conn != nil {
				_ = conn.Close()
			}
			require.NotNil(t, resp)
			assert.NotEqual(t, http.StatusSwitchingProtocols, resp.StatusCode)
		})

		t.Run("Origin not in the allowlist", func(t *testing.T) {
			header := http.Header{}
			header.Set(testProfileIDHeader, testProfileID.String())
			header.Set("Origin", "http://evil.example.com")
			conn, resp, err := websocket.DefaultDialer.Dial(wsURL, header)
			require.Error(t, err)
			if conn != nil {
				_ = conn.Close()
			}
			require.NotNil(t, resp)
			assert.NotEqual(t, http.StatusSwitchingProtocols, resp.StatusCode)
		})
	})

	// NATS-gated: exercises the genuine publish -> NATS -> subscribe ->
	// forward path end to end, so it needs a reachable local (or CI)
	// NATS server — skips gracefully via RequireTestNATS otherwise.
	t.Run("forwards live tally and winner updates to a connected participant", func(t *testing.T) {
		testhelpers.RequireTestNATS(t, testNATS)
		testhelpers.TruncateAll(t, testDB)
		seedTestUser(t)
		bob := seedTestProfile(t, "Bob")
		group := seedTestGroup(t, nil, testProfileID, bob.ID)
		content1 := seedTestContent(t)
		content2 := seedTestContent(t)
		seedActiveWatchlistItem(t, testProfileID, content1.ID)
		seedActiveWatchlistItem(t, bob.ID, content2.ID)

		session := postCreateSession(t, group.ID, dto.CreateSessionRequest{
			Method:              "majority",
			ParticipantIDs:      []uuid.UUID{testProfileID, bob.ID},
			CandidateContentIDs: []uuid.UUID{content1.ID, content2.ID},
		}, http.StatusCreated)

		httpServer := httptest.NewServer(testRouter)
		defer httpServer.Close()

		wsURL := "ws" + strings.TrimPrefix(httpServer.URL, "http") + "/api/v1/sessions/" + session.ID.String() + "/live"
		header := http.Header{}
		header.Set(testProfileIDHeader, testProfileID.String())
		header.Set("Origin", testClientOrigin)
		conn, _, err := websocket.DefaultDialer.Dial(wsURL, header)
		require.NoError(t, err)
		defer func() { _ = conn.Close() }()

		// The Dial call returns once the server has written the HTTP 101
		// upgrade response, which happens before the handler reaches
		// ChanSubscribe — a fixed sleep here would be a flaky guess at how
		// long that takes. Poll instead: publish canaries on the session's
		// own subject via the real NATS connection until the WS client
		// actually receives one, which is only possible once ChanSubscribe
		// has genuinely completed server-side.
		waitForLiveSubscription(t, conn, session.ID)

		// Cast a vote over a normal HTTP request on a separate connection.
		postSessionAction(t, session.ID, "votes", dto.CastVoteRequest{ContentID: content1.ID}, "", http.StatusOK)

		require.NoError(t, conn.SetReadDeadline(time.Now().Add(5*time.Second)))
		_, tallyFrame, err := conn.ReadMessage()
		require.NoError(t, err)

		var tallyMsg struct {
			Type  string `json:"type"`
			Tally struct {
				Counts map[string]int `json:"counts"`
			} `json:"tally"`
		}
		require.NoError(t, json.Unmarshal(tallyFrame, &tallyMsg))
		assert.Equal(t, "tally", tallyMsg.Type)
		assert.Equal(t, 1, tallyMsg.Tally.Counts[content1.ID.String()])

		// Second participant votes the same way so the winner is
		// unambiguous, then finalize.
		postSessionAction(t, session.ID, "votes", dto.CastVoteRequest{ContentID: content1.ID}, bob.ID.String(), http.StatusOK)

		// Drain the second vote's own tally broadcast before finalizing, so
		// it isn't mistaken for the winner frame below.
		require.NoError(t, conn.SetReadDeadline(time.Now().Add(5*time.Second)))
		_, _, err = conn.ReadMessage()
		require.NoError(t, err)

		finalized := postFinalize(t, session.ID, "", http.StatusOK)
		require.NotNil(t, finalized.WinnerContentID)

		require.NoError(t, conn.SetReadDeadline(time.Now().Add(5*time.Second)))
		_, winnerFrame, err := conn.ReadMessage()
		require.NoError(t, err)

		var winnerMsg dto.LiveWinnerMessage
		require.NoError(t, json.Unmarshal(winnerFrame, &winnerMsg))
		assert.Equal(t, "winner", winnerMsg.Type)
		assert.Equal(t, *finalized.WinnerContentID, winnerMsg.WinnerContentID)
		assert.False(t, winnerMsg.FinalizedAt.IsZero())
	})
}

// mustDataEnvelope wraps env in the {"data": ...} shape every handler in
// this codebase responds with, so callers can json.Unmarshal the raw POST
// body straight into it.
func mustDataEnvelope(env any) *struct {
	Data any `json:"data"`
} {
	return &struct {
		Data any `json:"data"`
	}{Data: env}
}
