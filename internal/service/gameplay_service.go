package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"edu-game-be/internal/domain"
	"edu-game-be/internal/repository"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

const (
	attemptTTL      = 30 * time.Minute
	mapCacheTTL     = 5 * time.Minute
	cacheKeyWorlds  = "cache:worlds:active"
	cacheKeyMapPref = "cache:world:map:"
)

type GameplayService struct {
	worldRepo    *repository.WorldRepository
	stageRepo    *repository.StageRepository
	progressRepo *repository.ProgressRepository
	redis        *redis.Client
}

func NewGameplayService(
	worldRepo *repository.WorldRepository,
	stageRepo *repository.StageRepository,
	progressRepo *repository.ProgressRepository,
	redisClient *redis.Client,
) *GameplayService {
	return &GameplayService{
		worldRepo:    worldRepo,
		stageRepo:    stageRepo,
		progressRepo: progressRepo,
		redis:        redisClient,
	}
}

type cachedWorldsBundle struct {
	Worlds      []domain.World    `json:"worlds"`
	StageCounts map[uuid.UUID]int `json:"stage_counts"`
}

type cachedWorldMapBundle struct {
	World  domain.World   `json:"world"`
	Stages []domain.Stage `json:"stages"`
}

func (s *GameplayService) ListWorlds(ctx context.Context, profileID uuid.UUID) ([]domain.WorldSummaryResponse, error) {
	bundle, err := s.getActiveWorldsCached(ctx)
	if err != nil {
		return nil, err
	}

	worldIDs := make([]uuid.UUID, 0, len(bundle.Worlds))
	for _, w := range bundle.Worlds {
		worldIDs = append(worldIDs, w.ID)
	}

	statsMap, err := s.progressRepo.GetWorldStatsForProfile(ctx, profileID, worldIDs)
	if err != nil {
		return nil, err
	}

	result := make([]domain.WorldSummaryResponse, 0, len(bundle.Worlds))
	for i, w := range bundle.Worlds {
		totalStages := bundle.StageCounts[w.ID]
		stats := statsMap[w.ID]
		isUnlocked := stats.UnlockedOrDone > 0 || i == 0

		result = append(result, domain.WorldSummaryResponse{
			ID:              w.ID,
			Code:            w.Code,
			Name:            w.Name,
			Description:     w.Description,
			OrderIndex:      w.OrderIndex,
			MapBgURL:        w.MapBgURL,
			BgmURL:          w.BgmURL,
			TotalStages:     totalStages,
			CompletedStages: stats.CompletedStages,
			EarnedStars:     stats.EarnedStars,
			MaxStars:        totalStages * 3,
			IsUnlocked:      isUnlocked,
		})
	}

	return result, nil
}

func (s *GameplayService) GetWorldMap(ctx context.Context, profileID, worldID uuid.UUID) (*domain.WorldMapResponse, error) {
	bundle, err := s.getWorldMapCached(ctx, worldID)
	if err != nil {
		return nil, err
	}
	if bundle == nil {
		return nil, WrapErr(ErrNotFound, "world not found")
	}

	stageIDs := make([]uuid.UUID, 0, len(bundle.Stages))
	for _, st := range bundle.Stages {
		stageIDs = append(stageIDs, st.ID)
	}

	progressMap, err := s.progressRepo.ListByProfileAndStageIDs(ctx, profileID, stageIDs)
	if err != nil {
		return nil, err
	}

	nodes := make([]domain.StageMapNodeResponse, 0, len(bundle.Stages))
	for _, st := range bundle.Stages {
		node := domain.StageMapNodeResponse{
			ID:           st.ID,
			OrderIndex:   st.OrderIndex,
			Name:         st.Name,
			ObstacleType: st.ObstacleType,
			PosX:         st.PosX,
			PosY:         st.PosY,
			RewardCoins:  st.RewardCoins,
			RewardItem:   st.RewardItem,
			Status:       domain.ProgressStatusLocked,
		}
		if prog, ok := progressMap[st.ID]; ok {
			node.Status = prog.Status
			node.Stars = prog.Stars
			node.BestWrongAttempts = prog.BestWrongAttempts
			node.CompletedAt = prog.CompletedAt
		}
		nodes = append(nodes, node)
	}

	return &domain.WorldMapResponse{
		World:  bundle.World,
		Stages: nodes,
	}, nil
}

func (s *GameplayService) StartStage(ctx context.Context, profileID, stageID uuid.UUID) (*domain.StartStageResponse, error) {
	stage, err := s.stageRepo.FindByID(ctx, stageID)
	if err != nil {
		return nil, err
	}
	if stage == nil {
		return nil, WrapErr(ErrNotFound, "stage not found")
	}

	progress, err := s.progressRepo.FindByProfileAndStage(ctx, profileID, stageID)
	if err != nil {
		return nil, err
	}
	if progress == nil || (progress.Status != domain.ProgressStatusUnlocked && progress.Status != domain.ProgressStatusCompleted) {
		return nil, WrapErr(ErrForbidden, "stage is locked for this profile")
	}

	challenges, err := s.stageRepo.ListChallengesByStageID(ctx, stageID)
	if err != nil {
		return nil, err
	}
	if len(challenges) == 0 {
		return nil, WrapErr(ErrNotFound, "stage has no challenges configured")
	}

	sanitized := make([]domain.SanitizedChallenge, 0, len(challenges))
	for _, ch := range challenges {
		sc, err := SanitizeChallenge(ch)
		if err != nil {
			return nil, fmt.Errorf("sanitize challenge %s: %w", ch.ID, err)
		}
		sanitized = append(sanitized, sc)
	}

	attemptID, err := generateAttemptID()
	if err != nil {
		return nil, fmt.Errorf("generate attempt id: %w", err)
	}

	attemptKey := attemptRedisKey(attemptID)
	pipe := s.redis.TxPipeline()
	pipe.HSet(ctx, attemptKey,
		"profile_id", profileID.String(),
		"stage_id", stageID.String(),
		"wrong_count", 0,
	)
	pipe.Expire(ctx, attemptKey, attemptTTL)
	if _, err := pipe.Exec(ctx); err != nil {
		return nil, fmt.Errorf("store attempt session: %w", err)
	}

	return &domain.StartStageResponse{
		AttemptID: attemptID,
		Stage: domain.StageStartInfo{
			ID:           stage.ID,
			Name:         stage.Name,
			ObstacleType: stage.ObstacleType,
		},
		Challenges: sanitized,
	}, nil
}

func (s *GameplayService) SubmitAnswer(ctx context.Context, profileID, stageID uuid.UUID, req domain.SubmitAnswerRequest) (*domain.SubmitAnswerResponse, error) {
	if strings.TrimSpace(req.AttemptID) == "" {
		return nil, WrapErr(ErrValidation, "attempt_id is required")
	}
	if req.ChallengeID == uuid.Nil {
		return nil, WrapErr(ErrValidation, "challenge_id is required")
	}
	if len(req.Answer) == 0 {
		return nil, WrapErr(ErrValidation, "answer is required")
	}

	attemptKey := attemptRedisKey(req.AttemptID)
	sessionData, err := s.redis.HGetAll(ctx, attemptKey).Result()
	if err != nil {
		return nil, fmt.Errorf("read attempt session: %w", err)
	}
	if len(sessionData) == 0 {
		return nil, WrapErr(ErrAttemptExpired, "attempt session not found or expired")
	}
	if sessionData["profile_id"] != profileID.String() || sessionData["stage_id"] != stageID.String() {
		return nil, WrapErr(ErrForbidden, "attempt does not match current profile or stage")
	}

	stage, err := s.stageRepo.FindByID(ctx, stageID)
	if err != nil {
		return nil, err
	}
	if stage == nil {
		return nil, WrapErr(ErrNotFound, "stage not found")
	}

	challenges, err := s.stageRepo.ListChallengesByStageID(ctx, stageID)
	if err != nil {
		return nil, err
	}

	var targetChallenge *domain.Challenge
	for i := range challenges {
		if challenges[i].ID == req.ChallengeID {
			targetChallenge = &challenges[i]
			break
		}
	}
	if targetChallenge == nil {
		return nil, WrapErr(ErrNotFound, "challenge does not belong to this stage")
	}

	isCorrect, hint, explanation, err := EvaluateChallengeAnswer(*targetChallenge, req.Answer)
	if err != nil {
		return nil, WrapErr(ErrValidation, err.Error())
	}

	if !isCorrect {
		wrongCount, err := s.redis.HIncrBy(ctx, attemptKey, "wrong_count", 1).Result()
		if err != nil {
			return nil, fmt.Errorf("increment wrong_count: %w", err)
		}
		wc := int(wrongCount)
		return &domain.SubmitAnswerResponse{
			IsCorrect:      false,
			Hint:           hint,
			WrongAttempts:  &wc,
			StageCompleted: false,
		}, nil
	}

	// Mark this challenge solved inside the attempt session
	solvedField := "solved:" + targetChallenge.ID.String()
	if err := s.redis.HSet(ctx, attemptKey, solvedField, "1").Err(); err != nil {
		return nil, fmt.Errorf("mark challenge solved: %w", err)
	}

	updatedSession, err := s.redis.HGetAll(ctx, attemptKey).Result()
	if err != nil {
		return nil, fmt.Errorf("reload attempt session: %w", err)
	}

	allSolved := true
	for _, ch := range challenges {
		if updatedSession["solved:"+ch.ID.String()] != "1" {
			allSolved = false
			break
		}
	}

	if !allSolved {
		return &domain.SubmitAnswerResponse{
			IsCorrect:      true,
			Explanation:    explanation,
			StageCompleted: false,
		}, nil
	}

	wrongCount, _ := strconv.Atoi(updatedSession["wrong_count"])
	starsEarned := CalculateStars(wrongCount)

	var (
		isFirstClear bool
		rewards      *domain.SubmitRewards
		unlocks      = &domain.SubmitUnlocks{}
	)

	err = s.progressRepo.DB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 1. Row-level lock on profile_progress to prevent double-submit race conditions
		progress, err := s.progressRepo.FindByProfileAndStageForUpdate(ctx, tx, profileID, stageID)
		if err != nil {
			return err
		}
		if progress == nil || (progress.Status != domain.ProgressStatusUnlocked && progress.Status != domain.ProgressStatusCompleted) {
			return WrapErr(ErrForbidden, "stage is locked for this profile")
		}

		// 2. Determine first-time clear & update progress + rewards
		isFirstClear = progress.Status != domain.ProgressStatusCompleted
		now := time.Now().UTC()

		if isFirstClear {
			progress.Status = domain.ProgressStatusCompleted
			progress.Stars = starsEarned
			progress.BestWrongAttempts = wrongCount
			progress.CompletedAt = &now

			if err := s.progressRepo.SaveProgressTx(ctx, tx, progress); err != nil {
				return err
			}
			if err := s.progressRepo.AddProfileRewardsTx(ctx, tx, profileID, stage.RewardCoins, starsEarned); err != nil {
				return err
			}

			var rewardItemSummary *domain.RewardItemSummary
			if stage.RewardItemID != nil {
				if err := s.progressRepo.GrantItemTx(ctx, tx, profileID, *stage.RewardItemID); err != nil {
					return err
				}
				if stage.RewardItem != nil {
					rewardItemSummary = &domain.RewardItemSummary{
						ID:      stage.RewardItem.ID,
						Name:    stage.RewardItem.Name,
						IconURL: stage.RewardItem.IconURL,
					}
				}
			}

			rewards = &domain.SubmitRewards{
				Coins: stage.RewardCoins,
				Item:  rewardItemSummary,
			}
		} else {
			// Replay: only upgrade stars / best_wrong_attempts; never grant coins/item again.
			starsDelta := 0
			updated := false
			if starsEarned > progress.Stars {
				starsDelta = starsEarned - progress.Stars
				progress.Stars = starsEarned
				updated = true
			}
			if wrongCount < progress.BestWrongAttempts {
				progress.BestWrongAttempts = wrongCount
				updated = true
			}
			if updated {
				if err := s.progressRepo.SaveProgressTx(ctx, tx, progress); err != nil {
					return err
				}
				if starsDelta > 0 {
					if err := s.progressRepo.AddProfileRewardsTx(ctx, tx, profileID, 0, starsDelta); err != nil {
						return err
					}
				}
			}
			rewards = &domain.SubmitRewards{
				Coins: 0,
				Item:  nil,
			}
		}

		// 3. Unlock next stage in the same world
		nextStage, err := s.stageRepo.FindNextInWorld(ctx, tx, stage.WorldID, stage.OrderIndex)
		if err != nil {
			return err
		}
		if nextStage != nil {
			if err := s.progressRepo.UnlockStageTx(ctx, tx, profileID, nextStage.ID); err != nil {
				return err
			}
			unlocks.NextStageID = &nextStage.ID
			return nil
		}

		// 4. Last stage in world completed -> unlock first stage of next active world
		currentWorld, err := s.worldRepo.FindByID(ctx, stage.WorldID)
		if err != nil {
			return err
		}
		if currentWorld == nil {
			return nil
		}

		nextWorld, err := s.worldRepo.FindNextActiveAfter(ctx, tx, currentWorld.OrderIndex)
		if err != nil {
			return err
		}
		if nextWorld == nil {
			return nil
		}

		firstNextWorldStage, err := s.stageRepo.FindFirstInWorld(ctx, tx, nextWorld.ID)
		if err != nil {
			return err
		}
		if firstNextWorldStage != nil {
			if err := s.progressRepo.UnlockStageTx(ctx, tx, profileID, firstNextWorldStage.ID); err != nil {
				return err
			}
			unlocks.NextStageID = &firstNextWorldStage.ID
		}
		unlocks.NextWorldID = &nextWorld.ID
		return nil
	})
	if err != nil {
		return nil, err
	}

	// 5. Clear Redis attempt session after successful transaction
	_ = s.redis.Del(ctx, attemptKey).Err()

	return &domain.SubmitAnswerResponse{
		IsCorrect:      true,
		Explanation:    explanation,
		StageCompleted: true,
		StarsEarned:    &starsEarned,
		FirstTimeClear: &isFirstClear,
		Rewards:        rewards,
		Unlocks:        unlocks,
	}, nil
}

// SanitizeChallenge removes all answer keys before sending challenges to clients.
func SanitizeChallenge(ch domain.Challenge) (domain.SanitizedChallenge, error) {
	out := domain.SanitizedChallenge{
		ID:             ch.ID,
		Type:           ch.Type,
		PromptText:     ch.PromptText,
		PromptAudioURL: ch.PromptAudioURL,
	}

	switch ch.Type {
	case domain.ChallengeTypeMultipleChoice, domain.ChallengeTypeAudioMatch:
		var payload domain.MultipleChoicePayload
		if err := json.Unmarshal(ch.Payload, &payload); err != nil {
			return out, err
		}
		out.Options = payload.Options
	case domain.ChallengeTypeDragDrop:
		var payload domain.DragDropPayload
		if err := json.Unmarshal(ch.Payload, &payload); err != nil {
			return out, err
		}
		out.Items = payload.Items
		out.Targets = payload.Targets
	default:
		return out, fmt.Errorf("unsupported challenge type %q", ch.Type)
	}

	return out, nil
}

// EvaluateChallengeAnswer checks the submitted answer against the stored JSONB payload.
func EvaluateChallengeAnswer(ch domain.Challenge, rawAnswer json.RawMessage) (isCorrect bool, hint string, explanation string, err error) {
	switch ch.Type {
	case domain.ChallengeTypeMultipleChoice, domain.ChallengeTypeAudioMatch:
		var payload domain.MultipleChoicePayload
		if err := json.Unmarshal(ch.Payload, &payload); err != nil {
			return false, "", "", fmt.Errorf("invalid stored challenge payload: %w", err)
		}
		var answer string
		if err := json.Unmarshal(rawAnswer, &answer); err != nil {
			return false, "", "", fmt.Errorf("answer must be a string option id")
		}
		if strings.TrimSpace(answer) == "" {
			return false, "", "", fmt.Errorf("answer cannot be empty")
		}
		correct := strings.EqualFold(strings.TrimSpace(answer), strings.TrimSpace(payload.CorrectOptionID))
		return correct, payload.Hint, payload.Explanation, nil

	case domain.ChallengeTypeDragDrop:
		var payload domain.DragDropPayload
		if err := json.Unmarshal(ch.Payload, &payload); err != nil {
			return false, "", "", fmt.Errorf("invalid stored drag_drop payload: %w", err)
		}
		var answerMap map[string]string
		if err := json.Unmarshal(rawAnswer, &answerMap); err != nil {
			return false, "", "", fmt.Errorf("answer for drag_drop must be a JSON object mapping target_id to item_id")
		}
		if len(answerMap) != len(payload.CorrectMap) {
			return false, payload.Hint, payload.Explanation, nil
		}
		for targetID, expectedItemID := range payload.CorrectMap {
			if strings.TrimSpace(answerMap[targetID]) != strings.TrimSpace(expectedItemID) {
				return false, payload.Hint, payload.Explanation, nil
			}
		}
		return true, payload.Hint, payload.Explanation, nil

	default:
		return false, "", "", fmt.Errorf("unsupported challenge type %q", ch.Type)
	}
}

// CalculateStars computes earned stars based on wrong attempt count.
// 0 wrong = 3 stars, 1 wrong = 2 stars, >= 2 wrong = 1 star.
func CalculateStars(wrongCount int) int {
	switch {
	case wrongCount <= 0:
		return 3
	case wrongCount == 1:
		return 2
	default:
		return 1
	}
}

func (s *GameplayService) getActiveWorldsCached(ctx context.Context) (*cachedWorldsBundle, error) {
	if s.redis != nil {
		if raw, err := s.redis.Get(ctx, cacheKeyWorlds).Bytes(); err == nil {
			var bundle cachedWorldsBundle
			if err := json.Unmarshal(raw, &bundle); err == nil {
				return &bundle, nil
			}
		}
	}

	worlds, err := s.worldRepo.ListActive(ctx)
	if err != nil {
		return nil, err
	}

	worldIDs := make([]uuid.UUID, 0, len(worlds))
	for _, w := range worlds {
		worldIDs = append(worldIDs, w.ID)
	}
	stageCounts, err := s.stageRepo.CountByWorldIDs(ctx, worldIDs)
	if err != nil {
		return nil, err
	}

	bundle := &cachedWorldsBundle{
		Worlds:      worlds,
		StageCounts: stageCounts,
	}

	if s.redis != nil {
		if encoded, err := json.Marshal(bundle); err == nil {
			_ = s.redis.Set(ctx, cacheKeyWorlds, encoded, mapCacheTTL).Err()
		}
	}
	return bundle, nil
}

func (s *GameplayService) getWorldMapCached(ctx context.Context, worldID uuid.UUID) (*cachedWorldMapBundle, error) {
	cacheKey := cacheKeyMapPref + worldID.String()
	if s.redis != nil {
		if raw, err := s.redis.Get(ctx, cacheKey).Bytes(); err == nil {
			var bundle cachedWorldMapBundle
			if err := json.Unmarshal(raw, &bundle); err == nil {
				return &bundle, nil
			}
		}
	}

	world, err := s.worldRepo.FindByID(ctx, worldID)
	if err != nil {
		return nil, err
	}
	if world == nil || !world.IsActive {
		return nil, nil
	}

	stages, err := s.stageRepo.ListByWorldID(ctx, worldID)
	if err != nil {
		return nil, err
	}

	bundle := &cachedWorldMapBundle{
		World:  *world,
		Stages: stages,
	}

	if s.redis != nil {
		if encoded, err := json.Marshal(bundle); err == nil {
			_ = s.redis.Set(ctx, cacheKey, encoded, mapCacheTTL).Err()
		}
	}
	return bundle, nil
}

func attemptRedisKey(attemptID string) string {
	return "attempt:" + attemptID
}

func generateAttemptID() (string, error) {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return "att_" + hex.EncodeToString(buf), nil
}
