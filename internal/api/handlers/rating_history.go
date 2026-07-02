package handlers

import (
	"context"
	"net/http"

	"github.com/bmstu-itstech/tjudge/internal/api/httputil"
	"github.com/bmstu-itstech/tjudge/internal/domain"
	"github.com/bmstu-itstech/tjudge/pkg/logger"
	"github.com/bmstu-itstech/tjudge/pkg/pagination"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// RatingHistoryRepository - доступ к истории рейтинга для графиков.
type RatingHistoryRepository interface {
	GetByProgramAndTournament(ctx context.Context, programID, tournamentID uuid.UUID, limit int) ([]*domain.RatingHistory, error)
}

// RatingHistoryHandler отдаёт историю рейтинга программы в турнире -
// данные для графика динамики (хронологический порядок).
type RatingHistoryHandler struct {
	repo RatingHistoryRepository
	log  *logger.Logger
}

func NewRatingHistoryHandler(repo RatingHistoryRepository, log *logger.Logger) *RatingHistoryHandler {
	return &RatingHistoryHandler{repo: repo, log: log}
}

// GetProgramRatingHistory возвращает историю рейтинга программы в турнире.
// @Summary История рейтинга программы
// @Description Хронология изменений ELO программы в турнире (для графика)
// @Tags tournaments
// @Produce json
// @Param id path string true "Tournament ID" format(uuid)
// @Param programId path string true "Program ID" format(uuid)
// @Param limit query int false "Максимум последних точек" default(200)
// @Success 200 {array} domain.RatingHistory
// @Failure 400 {object} object{error=string}
// @Router /tournaments/{id}/programs/{programId}/rating-history [get]
func (h *RatingHistoryHandler) GetProgramRatingHistory(w http.ResponseWriter, r *http.Request) {
	tournamentID, ok := httputil.ParseUUIDParam(w, r, "id", "tournament")
	if !ok {
		return
	}
	programID, ok := httputil.ParseUUIDParam(w, r, "programId", "program")
	if !ok {
		return
	}

	pg := pagination.ParseLimitOffset(r, 200, 0)

	history, err := h.repo.GetByProgramAndTournament(r.Context(), programID, tournamentID, pg.Limit)
	if err != nil {
		h.log.LogError("Failed to get program rating history", err,
			zap.String("tournament_id", tournamentID.String()),
			zap.String("program_id", programID.String()),
		)
		writeError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, history)
}
