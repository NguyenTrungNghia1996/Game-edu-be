package service

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"edu-game-be/internal/config"
	"edu-game-be/internal/domain"

	"github.com/google/uuid"
	"gorm.io/datatypes"
)

func TestSanitizeChallengeNeverLeaksAnswers(t *testing.T) {
	mcRaw, _ := json.Marshal(domain.MultipleChoicePayload{
		Options: []domain.ChoiceOption{
			{ID: "A", Text: "Chân thành", AudioURL: "https://assets.example.com/a.mp3"},
			{ID: "B", Text: "Trân thành"},
		},
		CorrectOptionID: "A",
		Hint:            "Nghe kỹ âm đầu Ch",
		Explanation:     "Chân thành nghĩa là thật lòng",
	})

	mcChallenge := domain.Challenge{
		ID:         uuid.New(),
		StageID:    uuid.New(),
		Type:       domain.ChallengeTypeMultipleChoice,
		PromptText: "Chọn từ đúng chính tả",
		Payload:    datatypes.JSON(mcRaw),
	}

	sanitized, err := SanitizeChallenge(mcChallenge)
	if err != nil {
		t.Fatalf("SanitizeChallenge failed: %v", err)
	}

	encoded, err := json.Marshal(sanitized)
	if err != nil {
		t.Fatalf("marshal sanitized challenge: %v", err)
	}
	jsonStr := string(encoded)

	for _, forbidden := range []string{"correct_option_id", "correct_map", "Nghe kỹ âm đầu Ch", "thật lòng"} {
		if strings.Contains(jsonStr, forbidden) {
			t.Fatalf("sanitized challenge leaked %q: %s", forbidden, jsonStr)
		}
	}
	if len(sanitized.Options) != 2 {
		t.Fatalf("expected 2 options, got %d", len(sanitized.Options))
	}

	ddRaw, _ := json.Marshal(domain.DragDropPayload{
		Items:      []domain.DragDropItem{{ID: "i1", Text: "r"}},
		Targets:    []domain.DragDropTarget{{ID: "t1", Label: "...ừng cây"}},
		CorrectMap: map[string]string{"t1": "i1"},
		Hint:       "Secret hint",
	})
	ddChallenge := domain.Challenge{
		ID:      uuid.New(),
		Type:    domain.ChallengeTypeDragDrop,
		Payload: datatypes.JSON(ddRaw),
	}
	sanitizedDD, err := SanitizeChallenge(ddChallenge)
	if err != nil {
		t.Fatalf("SanitizeChallenge drag_drop failed: %v", err)
	}
	ddEncoded, _ := json.Marshal(sanitizedDD)
	if strings.Contains(string(ddEncoded), "correct_map") || strings.Contains(string(ddEncoded), "Secret hint") {
		t.Fatalf("drag_drop sanitized challenge leaked secret fields: %s", string(ddEncoded))
	}
	if len(sanitizedDD.Items) != 1 || len(sanitizedDD.Targets) != 1 {
		t.Fatalf("expected 1 item and 1 target, got %d/%d", len(sanitizedDD.Items), len(sanitizedDD.Targets))
	}
}

func TestEvaluateChallengeAnswer(t *testing.T) {
	mcRaw, _ := json.Marshal(domain.MultipleChoicePayload{
		Options:         []domain.ChoiceOption{{ID: "A", Text: "Chân thành"}, {ID: "B", Text: "Trân thành"}},
		CorrectOptionID: "A",
		Hint:            "Chú ý âm Ch",
		Explanation:     "Đúng rồi!",
	})
	mc := domain.Challenge{Type: domain.ChallengeTypeMultipleChoice, Payload: datatypes.JSON(mcRaw)}

	ok, _, explanation, err := EvaluateChallengeAnswer(mc, json.RawMessage(`"A"`))
	if err != nil || !ok || explanation != "Đúng rồi!" {
		t.Fatalf("expected correct answer, got ok=%v expl=%q err=%v", ok, explanation, err)
	}

	ok, hint, _, err := EvaluateChallengeAnswer(mc, json.RawMessage(`"B"`))
	if err != nil || ok || hint != "Chú ý âm Ch" {
		t.Fatalf("expected wrong answer with hint, got ok=%v hint=%q err=%v", ok, hint, err)
	}

	ddRaw, _ := json.Marshal(domain.DragDropPayload{
		Items:       []domain.DragDropItem{{ID: "i1", Text: "r"}, {ID: "i2", Text: "d"}},
		Targets:     []domain.DragDropTarget{{ID: "t1", Label: "rừng"}, {ID: "t2", Label: "dòng"}},
		CorrectMap:  map[string]string{"t1": "i1", "t2": "i2"},
		Hint:        "Thử lại nhé",
		Explanation: "Chính xác!",
	})
	dd := domain.Challenge{Type: domain.ChallengeTypeDragDrop, Payload: datatypes.JSON(ddRaw)}

	ok, _, explanation, err = EvaluateChallengeAnswer(dd, json.RawMessage(`{"t1":"i1","t2":"i2"}`))
	if err != nil || !ok || explanation != "Chính xác!" {
		t.Fatalf("expected correct drag_drop, got ok=%v expl=%q err=%v", ok, explanation, err)
	}

	ok, hint, _, err = EvaluateChallengeAnswer(dd, json.RawMessage(`{"t1":"i2","t2":"i1"}`))
	if err != nil || ok || hint != "Thử lại nhé" {
		t.Fatalf("expected wrong drag_drop with hint, got ok=%v hint=%q err=%v", ok, hint, err)
	}
}

func TestCalculateStars(t *testing.T) {
	cases := []struct {
		wrong int
		want  int
	}{
		{0, 3},
		{1, 2},
		{2, 1},
		{5, 1},
	}
	for _, tc := range cases {
		if got := CalculateStars(tc.wrong); got != tc.want {
			t.Errorf("CalculateStars(%d) = %d, want %d", tc.wrong, got, tc.want)
		}
	}
}

func TestValidateChallengePayload(t *testing.T) {
	validMC := json.RawMessage(`{"options":[{"id":"A","text":"1"},{"id":"B","text":"2"}],"correct_option_id":"A"}`)
	if err := validateChallengePayload(domain.ChallengeTypeMultipleChoice, validMC); err != nil {
		t.Fatalf("expected valid payload, got %v", err)
	}

	invalidMC := json.RawMessage(`{"options":[{"id":"A","text":"1"},{"id":"B","text":"2"}],"correct_option_id":"Z"}`)
	if err := validateChallengePayload(domain.ChallengeTypeMultipleChoice, invalidMC); err == nil {
		t.Fatalf("expected error for mismatched correct_option_id")
	}

	invalidType := json.RawMessage(`{}`)
	if err := validateChallengePayload("unknown", invalidType); err == nil {
		t.Fatalf("expected error for unknown challenge type")
	}
}

func TestJWTTokenGenerateAndParse(t *testing.T) {
	svc := &AuthService{
		cfg: &config.Config{
			JWTSecret:             "super_secret_test_key_with_32_plus_chars",
			JWTExpireHours:        1,
			JWTRefreshExpireHours: 24,
		},
	}
	user := domain.User{
		ID:    uuid.New(),
		Email: "parent@test.local",
		Role:  domain.RoleParent,
	}

	accessToken, err := svc.generateToken(user, "access", time.Hour)
	if err != nil {
		t.Fatalf("generate access token: %v", err)
	}
	refreshToken, err := svc.generateToken(user, "refresh", 24*time.Hour)
	if err != nil {
		t.Fatalf("generate refresh token: %v", err)
	}

	claims, err := ParseToken(accessToken, svc.cfg.JWTSecret, "access")
	if err != nil {
		t.Fatalf("parse access token: %v", err)
	}
	if claims.UserID != user.ID.String() || claims.Role != domain.RoleParent {
		t.Fatalf("unexpected claims: %+v", claims)
	}

	if _, err := ParseToken(refreshToken, svc.cfg.JWTSecret, "access"); err == nil {
		t.Fatalf("expected refresh token to be rejected when access token is required")
	}
	if _, err := ParseToken(accessToken, "wrong_secret_key_for_testing_purposes", "access"); err == nil {
		t.Fatalf("expected invalid signature error")
	}
}
