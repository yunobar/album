package handler

import (
	"net/http"
	"slices"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	_ "github.com/itsLeonB/ginkgo/pkg/response"
	"github.com/itsLeonB/ginkgo/pkg/server"
	"github.com/nats-io/nats.go"
	"github.com/yunobar/album/internal/appconstant"
	"github.com/yunobar/album/internal/core/config"
	"github.com/yunobar/album/internal/core/logger"
	"github.com/yunobar/album/internal/core/pubsub"
	"github.com/yunobar/album/internal/domain/dto"
	"github.com/yunobar/album/internal/domain/service"
)

type DecisionSessionHandler struct {
	decisionSessionService service.DecisionSessionService
	natsConn               *nats.Conn
}

func NewDecisionSessionHandler(decisionSessionService service.DecisionSessionService, natsConn *nats.Conn) *DecisionSessionHandler {
	return &DecisionSessionHandler{decisionSessionService, natsConn}
}

const (
	pongWait   = 60 * time.Second
	pingPeriod = (pongWait * 9) / 10
)

var wsUpgrader = websocket.Upgrader{
	// WS handshakes get no CORS preflight and the browser WebSocket
	// constructor can't set custom headers, so header-based CSRF defenses
	// used elsewhere can't apply here. Check Origin against the same
	// allowlist used for CORS instead.
	CheckOrigin: func(r *http.Request) bool {
		origin := r.Header.Get("Origin")
		if origin == "" {
			return false
		}
		return slices.Contains(config.Global.ClientUrls, origin)
	},
}

// HandleCreate godoc
// @Summary      Create a decision session
// @Tags         sessions
// @Security     BearerAuth
// @Accept       json
// @Produce      json
// @Param        groupID path string true "Group ID"
// @Param        body body dto.CreateSessionRequest true "Create session payload"
// @Success      201  {object}  response.JSONResponse[dto.SessionResponse]
// @Failure      400  {object}  map[string]any
// @Failure      404  {object}  map[string]any
// @Router       /groups/{groupID}/sessions [post]
func (dsh *DecisionSessionHandler) HandleCreate() gin.HandlerFunc {
	return server.Handler("DecisionSessionHandler.HandleCreate", http.StatusCreated, func(ctx *gin.Context) (any, error) {
		profileID, err := getProfileID(ctx)
		if err != nil {
			return nil, err
		}

		groupID, err := server.GetRequiredPathParam[uuid.UUID](ctx, appconstant.ContextGroupID.String())
		if err != nil {
			return nil, err
		}

		request, err := server.BindJSON[dto.CreateSessionRequest](ctx)
		if err != nil {
			return nil, err
		}

		return dsh.decisionSessionService.Create(ctx.Request.Context(), profileID, groupID, request)
	})
}

// HandleGet godoc
// @Summary      Get a decision session's detail
// @Tags         sessions
// @Security     BearerAuth
// @Produce      json
// @Param        sessionID path string true "Session ID"
// @Success      200  {object}  response.JSONResponse[dto.SessionResponse]
// @Failure      404  {object}  map[string]any
// @Router       /sessions/{sessionID} [get]
func (dsh *DecisionSessionHandler) HandleGet() gin.HandlerFunc {
	return server.Handler("DecisionSessionHandler.HandleGet", http.StatusOK, func(ctx *gin.Context) (any, error) {
		profileID, err := getProfileID(ctx)
		if err != nil {
			return nil, err
		}

		sessionID, err := server.GetRequiredPathParam[uuid.UUID](ctx, appconstant.ContextSessionID.String())
		if err != nil {
			return nil, err
		}

		return dsh.decisionSessionService.Get(ctx.Request.Context(), profileID, sessionID)
	})
}

// HandleCastVote godoc
// @Summary      Cast or replace a Majority-method vote
// @Tags         sessions
// @Security     BearerAuth
// @Accept       json
// @Produce      json
// @Param        sessionID path string true "Session ID"
// @Param        body body dto.CastVoteRequest true "Cast vote payload"
// @Success      200  {object}  response.JSONResponse[dto.TallyResponse]
// @Failure      400  {object}  map[string]any
// @Failure      404  {object}  map[string]any
// @Failure      409  {object}  map[string]any
// @Router       /sessions/{sessionID}/votes [post]
func (dsh *DecisionSessionHandler) HandleCastVote() gin.HandlerFunc {
	return server.Handler("DecisionSessionHandler.HandleCastVote", http.StatusOK, func(ctx *gin.Context) (any, error) {
		profileID, err := getProfileID(ctx)
		if err != nil {
			return nil, err
		}

		sessionID, err := server.GetRequiredPathParam[uuid.UUID](ctx, appconstant.ContextSessionID.String())
		if err != nil {
			return nil, err
		}

		request, err := server.BindJSON[dto.CastVoteRequest](ctx)
		if err != nil {
			return nil, err
		}

		return dsh.decisionSessionService.CastVote(ctx.Request.Context(), profileID, sessionID, request)
	})
}

// HandleSubmitRanking godoc
// @Summary      Submit or replace a Ranked-Choice ballot
// @Tags         sessions
// @Security     BearerAuth
// @Accept       json
// @Produce      json
// @Param        sessionID path string true "Session ID"
// @Param        body body dto.SubmitRankingRequest true "Submit ranking payload"
// @Success      200  {object}  response.JSONResponse[dto.TallyResponse]
// @Failure      400  {object}  map[string]any
// @Failure      404  {object}  map[string]any
// @Failure      409  {object}  map[string]any
// @Router       /sessions/{sessionID}/rankings [post]
func (dsh *DecisionSessionHandler) HandleSubmitRanking() gin.HandlerFunc {
	return server.Handler("DecisionSessionHandler.HandleSubmitRanking", http.StatusOK, func(ctx *gin.Context) (any, error) {
		profileID, err := getProfileID(ctx)
		if err != nil {
			return nil, err
		}

		sessionID, err := server.GetRequiredPathParam[uuid.UUID](ctx, appconstant.ContextSessionID.String())
		if err != nil {
			return nil, err
		}

		request, err := server.BindJSON[dto.SubmitRankingRequest](ctx)
		if err != nil {
			return nil, err
		}

		return dsh.decisionSessionService.SubmitRanking(ctx.Request.Context(), profileID, sessionID, request)
	})
}

// HandleSelect godoc
// @Summary      Round Robin chooser pick
// @Tags         sessions
// @Security     BearerAuth
// @Accept       json
// @Produce      json
// @Param        sessionID path string true "Session ID"
// @Param        body body dto.CastVoteRequest true "Select payload"
// @Success      200  {object}  response.JSONResponse[dto.TallyResponse]
// @Failure      400  {object}  map[string]any
// @Failure      403  {object}  map[string]any
// @Failure      404  {object}  map[string]any
// @Failure      409  {object}  map[string]any
// @Router       /sessions/{sessionID}/select [post]
func (dsh *DecisionSessionHandler) HandleSelect() gin.HandlerFunc {
	return server.Handler("DecisionSessionHandler.HandleSelect", http.StatusOK, func(ctx *gin.Context) (any, error) {
		profileID, err := getProfileID(ctx)
		if err != nil {
			return nil, err
		}

		sessionID, err := server.GetRequiredPathParam[uuid.UUID](ctx, appconstant.ContextSessionID.String())
		if err != nil {
			return nil, err
		}

		request, err := server.BindJSON[dto.CastVoteRequest](ctx)
		if err != nil {
			return nil, err
		}

		return dsh.decisionSessionService.Select(ctx.Request.Context(), profileID, sessionID, request)
	})
}

// HandleFinalize godoc
// @Summary      Finalize a decision session and lock in the winner
// @Tags         sessions
// @Security     BearerAuth
// @Produce      json
// @Param        sessionID path string true "Session ID"
// @Success      200  {object}  response.JSONResponse[dto.SessionResponse]
// @Failure      404  {object}  map[string]any
// @Failure      409  {object}  map[string]any
// @Router       /sessions/{sessionID}/finalize [post]
func (dsh *DecisionSessionHandler) HandleFinalize() gin.HandlerFunc {
	return server.Handler("DecisionSessionHandler.HandleFinalize", http.StatusOK, func(ctx *gin.Context) (any, error) {
		profileID, err := getProfileID(ctx)
		if err != nil {
			return nil, err
		}

		sessionID, err := server.GetRequiredPathParam[uuid.UUID](ctx, appconstant.ContextSessionID.String())
		if err != nil {
			return nil, err
		}

		return dsh.decisionSessionService.Finalize(ctx.Request.Context(), profileID, sessionID)
	})
}

// HandleLive godoc
// @Summary      Live tally/winner updates for a session
// @Tags         sessions
// @Security     BearerAuth
// @Param        sessionID path string true "Session ID"
// @Router       /sessions/{sessionID}/live [get]
func (dsh *DecisionSessionHandler) HandleLive() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		profileID, err := getProfileID(ctx)
		if err != nil {
			_ = ctx.Error(err)
			return
		}
		sessionID, err := server.GetRequiredPathParam[uuid.UUID](ctx, appconstant.ContextSessionID.String())
		if err != nil {
			_ = ctx.Error(err)
			return
		}
		if err := dsh.decisionSessionService.VerifyParticipant(ctx.Request.Context(), profileID, sessionID); err != nil {
			_ = ctx.Error(err)
			return
		}

		conn, err := wsUpgrader.Upgrade(ctx.Writer, ctx.Request, nil)
		if err != nil {
			return // Upgrade already wrote its own HTTP error response
		}
		defer func() {
			if err := conn.Close(); err != nil {
				logger.Errorf("error closing decision session live websocket for session %s: %v", sessionID, err)
			}
		}()

		// Keepalive: without this, a connection silently dropped by a NAT
		// or an idle-timing-out proxy/load balancer between the client and
		// this server never surfaces as a read/write error, and the read-
		// pump goroutine below blocks on ReadMessage() forever — a
		// per-connection leak. The read deadline only advances on a pong,
		// so a client that stops responding gets disconnected within
		// pongWait instead of held open indefinitely.
		_ = conn.SetReadDeadline(time.Now().Add(pongWait))
		conn.SetPongHandler(func(string) error {
			return conn.SetReadDeadline(time.Now().Add(pongWait))
		})

		msgCh := make(chan *nats.Msg, 16)
		sub, err := dsh.natsConn.ChanSubscribe(pubsub.LiveSubject(sessionID), msgCh)
		if err != nil {
			return
		}
		defer func() {
			if err := sub.Unsubscribe(); err != nil {
				logger.Errorf("error unsubscribing from decision session live channel for session %s: %v", sessionID, err)
			}
		}()

		// This channel is server -> client only (no client input expected);
		// a background read loop's only job is detecting when the client
		// closes the connection (or stops responding to pings — see the
		// read deadline above), so the write loop below can stop.
		disconnected := make(chan struct{})
		go func() {
			defer close(disconnected)
			for {
				if _, _, err := conn.ReadMessage(); err != nil {
					return
				}
			}
		}()

		// pingPeriod must stay well under pongWait so a ping always has
		// time to round-trip before the read deadline it's meant to refresh
		// would otherwise expire.
		ticker := time.NewTicker(pingPeriod)
		defer ticker.Stop()

		for {
			select {
			case <-disconnected:
				return
			case msg, ok := <-msgCh:
				if !ok {
					return
				}
				if err := conn.WriteMessage(websocket.TextMessage, msg.Data); err != nil {
					return
				}
			case <-ticker.C:
				if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
					return
				}
			}
		}
	}
}
