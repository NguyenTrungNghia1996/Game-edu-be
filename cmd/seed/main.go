package main

import (
	"context"
	"encoding/json"
	"log"
	"strings"
	"time"

	"edu-game-be/internal/config"
	"edu-game-be/internal/database"
	"edu-game-be/internal/domain"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/datatypes"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type stageSeed struct {
	OrderIndex   int
	Name         string
	ObstacleType string
	PosX         int
	PosY         int
	RewardCode   string
	RewardCoins  int
	ChallengeTyp string
	PromptText   string
	AudioPath    string
	Payload      any
}

type worldSeed struct {
	World  domain.World
	Stages []stageSeed
}

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	db, err := database.NewPostgres(cfg)
	if err != nil {
		log.Fatalf("connect postgres: %v", err)
	}

	if err := database.RunSQLMigrations(db, cfg.MigrationsDir); err != nil {
		log.Fatalf("run migrations: %v", err)
	}

	assetBase := cfg.S3PublicBaseURL
	if assetBase == "" {
		assetBase = "https://assets.yourdomain.com"
	}

	if err := seedAll(db, strings.TrimRight(assetBase, "/")); err != nil {
		log.Fatalf("seed failed: %v", err)
	}

	// Clear world/map cache in Redis so the API reflects newly seeded worlds immediately.
	if redisClient, err := database.NewRedis(cfg); err == nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		_ = redisClient.Del(ctx, "cache:worlds:active").Err()
		var cursor uint64
		for {
			keys, nextCursor, scanErr := redisClient.Scan(ctx, cursor, "cache:world:map:*", 100).Result()
			if scanErr != nil {
				break
			}
			if len(keys) > 0 {
				_ = redisClient.Del(ctx, keys...).Err()
			}
			cursor = nextCursor
			if cursor == 0 {
				break
			}
		}
		cancel()
		_ = redisClient.Close()
		log.Println("Cleared Redis world/map caches")
	}

	log.Println("Seed completed successfully! (5 worlds, 50 stages, 18 reward items)")
	log.Println("Demo Admin:  admin@edugame.local  / Admin123!")
	log.Println("Demo Parent: parent@edugame.local / Parent123! (Profile: Bé Bo)")
}

func seedAll(db *gorm.DB, assetBase string) error {
	return db.Transaction(func(tx *gorm.DB) error {
		// 1. Seed Users (Admin + Parent)
		if _, err := ensureUser(tx, "admin@edugame.local", "Admin123!", domain.RoleAdmin); err != nil {
			return err
		}
		parentUser, err := ensureUser(tx, "parent@edugame.local", "Parent123!", domain.RoleParent)
		if err != nil {
			return err
		}

		// 2. Seed Reward Items
		itemDefs := buildItemSeeds(assetBase)
		itemsByCode := make(map[string]domain.Item, len(itemDefs))
		for _, item := range itemDefs {
			saved, err := ensureItem(tx, item)
			if err != nil {
				return err
			}
			itemsByCode[saved.Code] = saved
		}

		// 3. Seed All Worlds & Stages
		worlds := buildWorldSeeds(assetBase)
		var firstStageWorld1ID uuid.UUID

		for _, ws := range worlds {
			savedWorld, err := ensureWorld(tx, ws.World)
			if err != nil {
				return err
			}

			for _, sd := range ws.Stages {
				var rewardItemID *uuid.UUID
				if sd.RewardCode != "" {
					if item, ok := itemsByCode[sd.RewardCode]; ok {
						id := item.ID
						rewardItemID = &id
					}
				}
				stage, err := ensureStage(tx, domain.Stage{
					WorldID:      savedWorld.ID,
					OrderIndex:   sd.OrderIndex,
					Name:         sd.Name,
					ObstacleType: sd.ObstacleType,
					PosX:         sd.PosX,
					PosY:         sd.PosY,
					RewardItemID: rewardItemID,
					RewardCoins:  sd.RewardCoins,
				})
				if err != nil {
					return err
				}
				if savedWorld.OrderIndex == 1 && sd.OrderIndex == 1 {
					firstStageWorld1ID = stage.ID
				}
				if err := ensureChallenge(tx, stage.ID, 1, sd.ChallengeTyp, sd.PromptText, assetBase+sd.AudioPath, sd.Payload); err != nil {
					return err
				}
			}
		}

		// 4. Seed demo Profile for parent user & unlock World 1 - Stage 1
		demoProfile, err := ensureProfile(tx, parentUser.ID, "Bé Bo", assetBase+"/avatars/boy_01.webp")
		if err != nil {
			return err
		}
		if firstStageWorld1ID != uuid.Nil {
			if err := tx.Clauses(clause.OnConflict{
				Columns:   []clause.Column{{Name: "profile_id"}, {Name: "stage_id"}},
				DoNothing: true,
			}).Create(&domain.ProfileProgress{
				ProfileID: demoProfile.ID,
				StageID:   firstStageWorld1ID,
				Status:    domain.ProgressStatusUnlocked,
			}).Error; err != nil {
				return err
			}
		}

		return nil
	})
}

func buildItemSeeds(assetBase string) []domain.Item {
	return []domain.Item{
		// World 1: Rừng Chữ Cái
		{Code: "wooden_hammer", Name: "Búa Gỗ Thần Kỳ", Description: "Dụng cụ sửa chữa cây cầu gỗ trong Rừng Chữ Cái.", Category: domain.ItemCategoryTool, IconURL: assetBase + "/items/wooden_hammer.webp", SpriteURL: assetBase + "/sprites/wooden_hammer.png"},
		{Code: "explorer_hat", Name: "Mũ Thám Hiểm Rừng Xanh", Description: "Chiếc mũ dành cho nhà thám hiểm nhí gan dạ.", Category: domain.ItemCategoryHat, IconURL: assetBase + "/items/explorer_hat.webp", SpriteURL: assetBase + "/sprites/explorer_hat.png"},
		{Code: "forest_cape", Name: "Áo Choàng Lá Xanh", Description: "Áo choàng phép thuật giúp băng qua khu rừng rậm.", Category: domain.ItemCategoryOutfit, IconURL: assetBase + "/items/forest_cape.webp", SpriteURL: assetBase + "/sprites/forest_cape.png"},
		{Code: "spelling_badge", Name: "Huy Hiệu Chính Tả Vàng", Description: "Phần thưởng cho bé vượt qua thử thách chính tả khó nhằn.", Category: domain.ItemCategoryBadge, IconURL: assetBase + "/items/spelling_badge.webp", SpriteURL: assetBase + "/sprites/spelling_badge.png"},
		{Code: "magic_lantern", Name: "Đèn Lồng Đom Đóm", Description: "Soi sáng con đường xuyên qua Hang Gấu Ngủ.", Category: domain.ItemCategoryTool, IconURL: assetBase + "/items/magic_lantern.webp", SpriteURL: assetBase + "/sprites/magic_lantern.png"},
		{Code: "golden_crown", Name: "Vương Miện Rừng Chữ Cái", Description: "Biểu tượng chiến thắng toàn bộ vùng đất Rừng Chữ Cái.", Category: domain.ItemCategoryHat, IconURL: assetBase + "/items/golden_crown.webp", SpriteURL: assetBase + "/sprites/golden_crown.png"},

		// World 2: Thung Lũng Con Số
		{Code: "abacus_wand", Name: "Gậy Bàn Tính Pha Lê", Description: "Chiếc gậy giúp giải nhanh mọi phép tính khó.", Category: domain.ItemCategoryTool, IconURL: assetBase + "/items/abacus_wand.webp", SpriteURL: assetBase + "/sprites/abacus_wand.png"},
		{Code: "math_robe", Name: "Áo Choàng Nhà Toán Học", Description: "Trang phục lấp lánh chữ số kỳ diệu.", Category: domain.ItemCategoryOutfit, IconURL: assetBase + "/items/math_robe.webp", SpriteURL: assetBase + "/sprites/math_robe.png"},
		{Code: "number_medal", Name: "Huy Chương Thần Đồng Số Học", Description: "Vinh danh nhà vô địch Thung Lũng Con Số.", Category: domain.ItemCategoryBadge, IconURL: assetBase + "/items/number_medal.webp", SpriteURL: assetBase + "/sprites/number_medal.png"},

		// World 3: Đảo Tiếng Anh Kỳ Diệu
		{Code: "pirate_hat", Name: "Mũ Hải Tặc Từ Vựng", Description: "Chiếc mũ oai phong trên Đảo Tiếng Anh Kỳ Diệu.", Category: domain.ItemCategoryHat, IconURL: assetBase + "/items/pirate_hat.webp", SpriteURL: assetBase + "/sprites/pirate_hat.png"},
		{Code: "compass_english", Name: "La Bàn Ngôn Ngữ", Description: "Dẫn đường khám phá kho báu từ vựng tiếng Anh.", Category: domain.ItemCategoryTool, IconURL: assetBase + "/items/compass_english.webp", SpriteURL: assetBase + "/sprites/compass_english.png"},
		{Code: "island_badge", Name: "Huy Hiệu Chinh Phục Đảo Anh Ngữ", Description: "Trao tặng cho bé hoàn thành Đảo Tiếng Anh Kỳ Diệu.", Category: domain.ItemCategoryBadge, IconURL: assetBase + "/items/island_badge.webp", SpriteURL: assetBase + "/sprites/island_badge.png"},

		// World 4: Đại Dương Khoa Học
		{Code: "diving_helmet", Name: "Mũ Thợ Lặn San Hô", Description: "Giúp bé lặn sâu khám phá bí ẩn dưới lòng đại dương.", Category: domain.ItemCategoryHat, IconURL: assetBase + "/items/diving_helmet.webp", SpriteURL: assetBase + "/sprites/diving_helmet.png"},
		{Code: "ocean_suit", Name: "Bộ Đồ Lặn Ánh Sáng", Description: "Trang phục bảo vệ nhà khoa học nhí dưới biển sâu.", Category: domain.ItemCategoryOutfit, IconURL: assetBase + "/items/ocean_suit.webp", SpriteURL: assetBase + "/sprites/ocean_suit.png"},
		{Code: "science_badge", Name: "Huy Hiệu Nhà Khoa Học Nhí", Description: "Ghi nhận sự am hiểu thiên nhiên và khoa học.", Category: domain.ItemCategoryBadge, IconURL: assetBase + "/items/science_badge.webp", SpriteURL: assetBase + "/sprites/science_badge.png"},

		// World 5: Vương Quốc Tư Duy
		{Code: "logic_spectacles", Name: "Kính Lúp Thám Tử", Description: "Nhìn thấu mọi quy luật ẩn giấu trong Vương Quốc Tư Duy.", Category: domain.ItemCategoryTool, IconURL: assetBase + "/items/logic_spectacles.webp", SpriteURL: assetBase + "/sprites/logic_spectacles.png"},
		{Code: "royal_armor", Name: "Giáp Hiệp Sĩ Trí Tuệ", Description: "Trang phục cao quý của hiệp sĩ giải đố.", Category: domain.ItemCategoryOutfit, IconURL: assetBase + "/items/royal_armor.webp", SpriteURL: assetBase + "/sprites/royal_armor.png"},
		{Code: "mastermind_crown", Name: "Vương Miện Bậc Thầy Tư Duy", Description: "Phần thưởng tối thượng của Vương Quốc Tư Duy.", Category: domain.ItemCategoryHat, IconURL: assetBase + "/items/mastermind_crown.webp", SpriteURL: assetBase + "/sprites/mastermind_crown.png"},
	}
}

func buildWorldSeeds(assetBase string) []worldSeed {
	return []worldSeed{
		// ============================================================
		// WORLD 1: RỪNG CHỮ CÁI (Tiếng Việt & Chính tả - 10 màn)
		// ============================================================
		{
			World: domain.World{
				Code:        "rung-chu-cai",
				Name:        "Rừng Chữ Cái",
				Description: "Hành trình chinh phục chính tả tiếng Việt và giải cứu khu rừng kỳ diệu.",
				OrderIndex:  1,
				MapBgURL:    assetBase + "/maps/rung-chu-cai-bg.webp",
				BgmURL:      assetBase + "/audio/bgm-rung-chu-cai.mp3",
				IsActive:    true,
			},
			Stages: []stageSeed{
				{
					OrderIndex: 1, Name: "Cây cầu gỗ bị gãy", ObstacleType: "broken_bridge",
					PosX: 140, PosY: 520, RewardCode: "wooden_hammer", RewardCoins: 50,
					ChallengeTyp: domain.ChallengeTypeMultipleChoice,
					PromptText:   "Chọn từ viết đúng chính tả để sửa cây cầu gỗ.",
					AudioPath:    "/audio/chl_01_prompt.mp3",
					Payload: domain.MultipleChoicePayload{
						Options: []domain.ChoiceOption{
							{ID: "A", Text: "Chân thành", AudioURL: assetBase + "/audio/opt_01_a.mp3"},
							{ID: "B", Text: "Trân thành", AudioURL: assetBase + "/audio/opt_01_b.mp3"},
							{ID: "C", Text: "Chân thàng", AudioURL: assetBase + "/audio/opt_01_c.mp3"},
						},
						CorrectOptionID: "A",
						Hint:            "Hãy lắng nghe âm đầu 'Ch' và vần 'anh' nhé!",
						Explanation:     "Chân thành (viết bằng Ch) có nghĩa là thật lòng.",
					},
				},
				{
					OrderIndex: 2, Name: "Cổng dây leo bí ẩn", ObstacleType: "locked_gate",
					PosX: 250, PosY: 460, RewardCoins: 50,
					ChallengeTyp: domain.ChallengeTypeMultipleChoice,
					PromptText:   "Chọn từ viết đúng chính tả (s / x).",
					AudioPath:    "/audio/chl_02_prompt.mp3",
					Payload: domain.MultipleChoicePayload{
						Options: []domain.ChoiceOption{
							{ID: "A", Text: "Chia xẻ"},
							{ID: "B", Text: "Chia sẻ"},
							{ID: "C", Text: "Tia sẻ"},
						},
						CorrectOptionID: "B",
						Hint:            "Từ 'sẻ' trong 'chia sẻ' viết bằng âm 's' cong lưỡi.",
						Explanation:     "Chia sẻ (viết bằng s) là cùng san sẻ niềm vui, nỗi buồn.",
					},
				},
				{
					OrderIndex: 3, Name: "Chú gấu đang ngủ quên", ObstacleType: "sleeping_bear",
					PosX: 360, PosY: 410, RewardCode: "explorer_hat", RewardCoins: 60,
					ChallengeTyp: domain.ChallengeTypeAudioMatch,
					PromptText:   "Nghe âm thanh và chọn con vật tương ứng.",
					AudioPath:    "/audio/chl_03_prompt.mp3",
					Payload: domain.MultipleChoicePayload{
						Options: []domain.ChoiceOption{
							{ID: "A", Text: "Con hươu", ImageURL: assetBase + "/images/deer.webp"},
							{ID: "B", Text: "Con hưu", ImageURL: assetBase + "/images/deer_alt.webp"},
							{ID: "C", Text: "Con khươu", ImageURL: assetBase + "/images/bird.webp"},
						},
						CorrectOptionID: "A",
						Hint:            "Vần 'ươu' gồm ư - ơ - u nhé bé!",
						Explanation:     "Con hươu viết đúng với vần 'ươu'.",
					},
				},
				{
					OrderIndex: 4, Name: "Dòng suối lấp lánh", ObstacleType: "broken_bridge",
					PosX: 470, PosY: 360, RewardCoins: 60,
					ChallengeTyp: domain.ChallengeTypeMultipleChoice,
					PromptText:   "Điền tiếng đúng vào chỗ trống: 'Dòng nước ... veo'.",
					AudioPath:    "/audio/chl_04_prompt.mp3",
					Payload: domain.MultipleChoicePayload{
						Options: []domain.ChoiceOption{
							{ID: "A", Text: "chong"},
							{ID: "B", Text: "trong"},
							{ID: "C", Text: "trông"},
						},
						CorrectOptionID: "B",
						Hint:            "Âm đầu 'tr' kết hợp với vần 'ong'.",
						Explanation:     "'Trong veo' chỉ dòng nước rất sạch và nhìn thấu đáy.",
					},
				},
				{
					OrderIndex: 5, Name: "Cánh cửa cổ thụ", ObstacleType: "locked_gate",
					PosX: 580, PosY: 320, RewardCode: "forest_cape", RewardCoins: 70,
					ChallengeTyp: domain.ChallengeTypeDragDrop,
					PromptText:   "Kéo đúng chữ cái đầu vào từng từ có nghĩa.",
					AudioPath:    "/audio/chl_05_prompt.mp3",
					Payload: domain.DragDropPayload{
						Items: []domain.DragDropItem{
							{ID: "item_d", Text: "d"},
							{ID: "item_gi", Text: "gi"},
							{ID: "item_r", Text: "r"},
						},
						Targets: []domain.DragDropTarget{
							{ID: "t1", Label: "... ừng cây"},
							{ID: "t2", Label: "... ió mát"},
							{ID: "t3", Label: "... òng sông"},
						},
						CorrectMap:  map[string]string{"t1": "item_r", "t2": "item_gi", "t3": "item_d"},
						Hint:        "Thử đọc to: 'rừng cây', 'gió mát', 'dòng sông' nhé!",
						Explanation: "Rừng cây (r), gió mát (gi), dòng sông (d).",
					},
				},
				{
					OrderIndex: 6, Name: "Vườn nấm phát sáng", ObstacleType: "sleeping_bear",
					PosX: 690, PosY: 290, RewardCoins: 70,
					ChallengeTyp: domain.ChallengeTypeMultipleChoice,
					PromptText:   "Chọn từ viết đúng dấu hỏi / ngã.",
					AudioPath:    "/audio/chl_06_prompt.mp3",
					Payload: domain.MultipleChoicePayload{
						Options: []domain.ChoiceOption{
							{ID: "A", Text: "Sữa chữa"},
							{ID: "B", Text: "Sửa chửa"},
							{ID: "C", Text: "Sửa chữa"},
						},
						CorrectOptionID: "C",
						Hint:            "'Sửa' dùng dấu hỏi, 'chữa' dùng dấu ngã.",
						Explanation:     "Sửa chữa viết đúng với dấu hỏi ở 'sửa' và dấu ngã ở 'chữa'.",
					},
				},
				{
					OrderIndex: 7, Name: "Hang đom đóm", ObstacleType: "locked_gate",
					PosX: 790, PosY: 340, RewardCode: "magic_lantern", RewardCoins: 80,
					ChallengeTyp: domain.ChallengeTypeMultipleChoice,
					PromptText:   "Chọn từ viết đúng quy tắc chính tả c / k / q.",
					AudioPath:    "/audio/chl_07_prompt.mp3",
					Payload: domain.MultipleChoicePayload{
						Options: []domain.ChoiceOption{
							{ID: "A", Text: "Con kiếng"},
							{ID: "B", Text: "Con kiến"},
							{ID: "C", Text: "Con ciến"},
						},
						CorrectOptionID: "B",
						Hint:            "Trước âm 'i', ta luôn dùng chữ 'k' và vần 'iên'.",
						Explanation:     "Con kiến dùng 'k' trước 'i' và vần 'iên'.",
					},
				},
				{
					OrderIndex: 8, Name: "Cầu treo mây trắng", ObstacleType: "broken_bridge",
					PosX: 880, PosY: 270, RewardCode: "spelling_badge", RewardCoins: 80,
					ChallengeTyp: domain.ChallengeTypeMultipleChoice,
					PromptText:   "Từ nào viết đúng chính tả (l / n)?",
					AudioPath:    "/audio/chl_08_prompt.mp3",
					Payload: domain.MultipleChoicePayload{
						Options: []domain.ChoiceOption{
							{ID: "A", Text: "Nấp lánh"},
							{ID: "B", Text: "Lấp lánh"},
							{ID: "C", Text: "Lấp nánh"},
						},
						CorrectOptionID: "B",
						Hint:            "Cả hai tiếng đều bắt đầu bằng chữ 'l'.",
						Explanation:     "Lấp lánh là từ láy phụ âm đầu 'l'.",
					},
				},
				{
					OrderIndex: 9, Name: "Cổng thành Gió Bấc", ObstacleType: "locked_gate",
					PosX: 960, PosY: 210, RewardCoins: 90,
					ChallengeTyp: domain.ChallengeTypeMultipleChoice,
					PromptText:   "Chọn từ viết đúng với quy tắc g / gh.",
					AudioPath:    "/audio/chl_09_prompt.mp3",
					Payload: domain.MultipleChoicePayload{
						Options: []domain.ChoiceOption{
							{ID: "A", Text: "Gế gỗ"},
							{ID: "B", Text: "Ghế ghỗ"},
							{ID: "C", Text: "Ghế gỗ"},
						},
						CorrectOptionID: "C",
						Hint:            "'gh' chỉ đứng trước e, ê, i.",
						Explanation:     "Ghế (trước ê dùng gh) và gỗ (trước ô dùng g).",
					},
				},
				{
					OrderIndex: 10, Name: "Đền Thờ Rừng Chữ Cái", ObstacleType: "sleeping_bear",
					PosX: 1050, PosY: 140, RewardCode: "golden_crown", RewardCoins: 120,
					ChallengeTyp: domain.ChallengeTypeMultipleChoice,
					PromptText:   "Thử thách cuối cùng! Chọn câu viết đúng hoàn toàn chính tả.",
					AudioPath:    "/audio/chl_10_prompt.mp3",
					Payload: domain.MultipleChoicePayload{
						Options: []domain.ChoiceOption{
							{ID: "A", Text: "Bé chăm chỉ học tập mỗi ngày."},
							{ID: "B", Text: "Bé trăm chỉ học tập mổi ngày."},
							{ID: "C", Text: "Bé chăm chĩ học tập mỗi ngày."},
						},
						CorrectOptionID: "A",
						Hint:            "Chú ý 'chăm chỉ' (ch, dấu hỏi) và 'mỗi' (dấu ngã).",
						Explanation:     "Xuất sắc! Bé đã hoàn thành Rừng Chữ Cái và mở khóa vùng đất mới!",
					},
				},
			},
		},

		// ============================================================
		// WORLD 2: THUNG LŨNG CON SỐ (Toán học - 10 màn)
		// ============================================================
		{
			World: domain.World{
				Code:        "thung-lung-con-so",
				Name:        "Thung Lũng Con Số",
				Description: "Khám phá phép tính vui nhộn cùng những người bạn số học.",
				OrderIndex:  2,
				MapBgURL:    assetBase + "/maps/thung-lung-con-so-bg.webp",
				BgmURL:      assetBase + "/audio/bgm-thung-lung-con-so.mp3",
				IsActive:    true,
			},
			Stages: []stageSeed{
				{
					OrderIndex: 1, Name: "Cánh cổng số học", ObstacleType: "locked_gate",
					PosX: 160, PosY: 480, RewardCoins: 60,
					ChallengeTyp: domain.ChallengeTypeMultipleChoice,
					PromptText:   "Tính nhanh: 7 + 5 bằng bao nhiêu?",
					AudioPath:    "/audio/w2_chl_01.mp3",
					Payload: domain.MultipleChoicePayload{
						Options:         []domain.ChoiceOption{{ID: "A", Text: "11"}, {ID: "B", Text: "12"}, {ID: "C", Text: "13"}},
						CorrectOptionID: "B",
						Hint:            "7 + 3 = 10, thêm 2 nữa là bao nhiêu nhỉ?",
						Explanation:     "7 + 5 = 12.",
					},
				},
				{
					OrderIndex: 2, Name: "Cầu đá phép trừ", ObstacleType: "broken_bridge",
					PosX: 270, PosY: 420, RewardCoins: 60,
					ChallengeTyp: domain.ChallengeTypeMultipleChoice,
					PromptText:   "Tính nhanh: 15 - 6 bằng bao nhiêu?",
					AudioPath:    "/audio/w2_chl_02.mp3",
					Payload: domain.MultipleChoicePayload{
						Options:         []domain.ChoiceOption{{ID: "A", Text: "8"}, {ID: "B", Text: "9"}, {ID: "C", Text: "10"}},
						CorrectOptionID: "B",
						Hint:            "15 - 5 = 10, bớt thêm 1 nữa nhé!",
						Explanation:     "15 - 6 = 9.",
					},
				},
				{
					OrderIndex: 3, Name: "Chú gấu đếm táo", ObstacleType: "sleeping_bear",
					PosX: 380, PosY: 360, RewardCode: "abacus_wand", RewardCoins: 70,
					ChallengeTyp: domain.ChallengeTypeMultipleChoice,
					PromptText:   "Mỗi giỏ có 3 quả táo. Hỏi 4 giỏ có tất cả bao nhiêu quả táo?",
					AudioPath:    "/audio/w2_chl_03.mp3",
					Payload: domain.MultipleChoicePayload{
						Options:         []domain.ChoiceOption{{ID: "A", Text: "10 quả"}, {ID: "B", Text: "12 quả"}, {ID: "C", Text: "14 quả"}},
						CorrectOptionID: "B",
						Hint:            "3 + 3 + 3 + 3 = ?",
						Explanation:     "3 x 4 = 12 quả táo.",
					},
				},
				{
					OrderIndex: 4, Name: "Thác nước phép chia", ObstacleType: "broken_bridge",
					PosX: 490, PosY: 320, RewardCoins: 70,
					ChallengeTyp: domain.ChallengeTypeMultipleChoice,
					PromptText:   "Có 18 viên kẹo chia đều cho 3 bạn. Mỗi bạn được mấy viên kẹo?",
					AudioPath:    "/audio/w2_chl_04.mp3",
					Payload: domain.MultipleChoicePayload{
						Options:         []domain.ChoiceOption{{ID: "A", Text: "5 viên"}, {ID: "B", Text: "6 viên"}, {ID: "C", Text: "7 viên"}},
						CorrectOptionID: "B",
						Hint:            "3 nhân với số nào thì bằng 18?",
						Explanation:     "18 : 3 = 6 viên kẹo.",
					},
				},
				{
					OrderIndex: 5, Name: "Khu vườn ghép phép tính", ObstacleType: "locked_gate",
					PosX: 600, PosY: 370, RewardCode: "math_robe", RewardCoins: 80,
					ChallengeTyp: domain.ChallengeTypeDragDrop,
					PromptText:   "Ghép mỗi phép tính với kết quả đúng.",
					AudioPath:    "/audio/w2_chl_05.mp3",
					Payload: domain.DragDropPayload{
						Items: []domain.DragDropItem{
							{ID: "v15", Text: "15"},
							{ID: "v20", Text: "20"},
							{ID: "v24", Text: "24"},
						},
						Targets: []domain.DragDropTarget{
							{ID: "t1", Label: "5 x 3 = ?"},
							{ID: "t2", Label: "4 x 5 = ?"},
							{ID: "t3", Label: "6 x 4 = ?"},
						},
						CorrectMap:  map[string]string{"t1": "v15", "t2": "v20", "t3": "v24"},
						Hint:        "Nhớ lại bảng cửu chương 3, 4, 5 nhé!",
						Explanation: "5 x 3 = 15, 4 x 5 = 20 và 6 x 4 = 24.",
					},
				},
				{
					OrderIndex: 6, Name: "Đỉnh đồi so sánh", ObstacleType: "sleeping_bear",
					PosX: 700, PosY: 300, RewardCoins: 80,
					ChallengeTyp: domain.ChallengeTypeMultipleChoice,
					PromptText:   "Số nào lớn nhất trong các số sau: 48, 84, 79?",
					AudioPath:    "/audio/w2_chl_06.mp3",
					Payload: domain.MultipleChoicePayload{
						Options:         []domain.ChoiceOption{{ID: "A", Text: "48"}, {ID: "B", Text: "84"}, {ID: "C", Text: "79"}},
						CorrectOptionID: "B",
						Hint:            "So sánh chữ số hàng chục trước nhé (4, 8, 7).",
						Explanation:     "84 có hàng chục là 8 nên lớn nhất.",
					},
				},
				{
					OrderIndex: 7, Name: "Cây cầu đồng hồ", ObstacleType: "broken_bridge",
					PosX: 800, PosY: 250, RewardCoins: 90,
					ChallengeTyp: domain.ChallengeTypeMultipleChoice,
					PromptText:   "1 giờ có bao nhiêu phút?",
					AudioPath:    "/audio/w2_chl_07.mp3",
					Payload: domain.MultipleChoicePayload{
						Options:         []domain.ChoiceOption{{ID: "A", Text: "30 phút"}, {ID: "B", Text: "60 phút"}, {ID: "C", Text: "100 phút"}},
						CorrectOptionID: "B",
						Hint:            "Kim phút quay đủ một vòng tròn trên đồng hồ là 60 vạch.",
						Explanation:     "1 giờ = 60 phút.",
					},
				},
				{
					OrderIndex: 8, Name: "Cánh cổng tìm X", ObstacleType: "locked_gate",
					PosX: 890, PosY: 310, RewardCoins: 90,
					ChallengeTyp: domain.ChallengeTypeMultipleChoice,
					PromptText:   "Tìm số còn thiếu: ... + 14 = 30",
					AudioPath:    "/audio/w2_chl_08.mp3",
					Payload: domain.MultipleChoicePayload{
						Options:         []domain.ChoiceOption{{ID: "A", Text: "14"}, {ID: "B", Text: "16"}, {ID: "C", Text: "18"}},
						CorrectOptionID: "B",
						Hint:            "Lấy 30 trừ đi 14 sẽ ra số cần tìm!",
						Explanation:     "30 - 14 = 16, nên 16 + 14 = 30.",
					},
				},
				{
					OrderIndex: 9, Name: "Hang động hình học", ObstacleType: "sleeping_bear",
					PosX: 980, PosY: 230, RewardCoins: 100,
					ChallengeTyp: domain.ChallengeTypeMultipleChoice,
					PromptText:   "Hình tam giác có bao nhiêu cạnh?",
					AudioPath:    "/audio/w2_chl_09.mp3",
					Payload: domain.MultipleChoicePayload{
						Options:         []domain.ChoiceOption{{ID: "A", Text: "3 cạnh"}, {ID: "B", Text: "4 cạnh"}, {ID: "C", Text: "5 cạnh"}},
						CorrectOptionID: "A",
						Hint:            "'Tam' nghĩa là ba đó bé!",
						Explanation:     "Hình tam giác luôn có 3 cạnh và 3 đỉnh.",
					},
				},
				{
					OrderIndex: 10, Name: "Lâu Đài Số Học", ObstacleType: "locked_gate",
					PosX: 1070, PosY: 150, RewardCode: "number_medal", RewardCoins: 130,
					ChallengeTyp: domain.ChallengeTypeMultipleChoice,
					PromptText:   "Thử thách cuối! Lan có 25 bông hoa, Lan tặng bạn 8 bông rồi hái thêm 5 bông. Hỏi Lan còn bao nhiêu bông hoa?",
					AudioPath:    "/audio/w2_chl_10.mp3",
					Payload: domain.MultipleChoicePayload{
						Options:         []domain.ChoiceOption{{ID: "A", Text: "20 bông"}, {ID: "B", Text: "22 bông"}, {ID: "C", Text: "24 bông"}},
						CorrectOptionID: "B",
						Hint:            "Tính 25 - 8 trước, sau đó cộng thêm 5 nhé!",
						Explanation:     "25 - 8 + 5 = 22 bông hoa. Chúc mừng bé chinh phục Thung Lũng Con Số!",
					},
				},
			},
		},

		// ============================================================
		// WORLD 3: ĐẢO TIẾNG ANH KỲ DIỆU (English - 10 màn)
		// ============================================================
		{
			World: domain.World{
				Code:        "dao-tieng-anh",
				Name:        "Đảo Tiếng Anh Kỳ Diệu",
				Description: "Ra khơi học từ vựng tiếng Anh quen thuộc qua âm thanh và hình ảnh sinh động.",
				OrderIndex:  3,
				MapBgURL:    assetBase + "/maps/dao-tieng-anh-bg.webp",
				BgmURL:      assetBase + "/audio/bgm-dao-tieng-anh.mp3",
				IsActive:    true,
			},
			Stages: []stageSeed{
				{
					OrderIndex: 1, Name: "Bãi cát chào hỏi", ObstacleType: "broken_bridge",
					PosX: 140, PosY: 510, RewardCoins: 60,
					ChallengeTyp: domain.ChallengeTypeMultipleChoice,
					PromptText:   "Từ 'Hello' trong tiếng Việt có nghĩa là gì?",
					AudioPath:    "/audio/w3_chl_01.mp3",
					Payload: domain.MultipleChoicePayload{
						Options:         []domain.ChoiceOption{{ID: "A", Text: "Xin chào"}, {ID: "B", Text: "Tạm biệt"}, {ID: "C", Text: "Cảm ơn"}},
						CorrectOptionID: "A",
						Hint:            "Đây là câu nói khi mới gặp ai đó!",
						Explanation:     "'Hello' nghĩa là Xin chào.",
					},
				},
				{
					OrderIndex: 2, Name: "Rừng dừa sắc màu", ObstacleType: "locked_gate",
					PosX: 250, PosY: 440, RewardCoins: 60,
					ChallengeTyp: domain.ChallengeTypeMultipleChoice,
					PromptText:   "Màu xanh dương trong tiếng Anh là gì?",
					AudioPath:    "/audio/w3_chl_02.mp3",
					Payload: domain.MultipleChoicePayload{
						Options:         []domain.ChoiceOption{{ID: "A", Text: "Red"}, {ID: "B", Text: "Blue"}, {ID: "C", Text: "Yellow"}},
						CorrectOptionID: "B",
						Hint:            "Màu của bầu trời và biển cả: B-l-u-e.",
						Explanation:     "Blue là màu xanh dương (Red: đỏ, Yellow: vàng).",
					},
				},
				{
					OrderIndex: 3, Name: "Vịnh thú cưng", ObstacleType: "sleeping_bear",
					PosX: 360, PosY: 390, RewardCode: "pirate_hat", RewardCoins: 70,
					ChallengeTyp: domain.ChallengeTypeAudioMatch,
					PromptText:   "Nghe phát âm tiếng Anh và chọn con vật đúng.",
					AudioPath:    "/audio/w3_chl_03_elephant.mp3",
					Payload: domain.MultipleChoicePayload{
						Options: []domain.ChoiceOption{
							{ID: "A", Text: "Elephant (Con voi)", ImageURL: assetBase + "/images/elephant.webp"},
							{ID: "B", Text: "Tiger (Con hổ)", ImageURL: assetBase + "/images/tiger.webp"},
							{ID: "C", Text: "Rabbit (Con thỏ)", ImageURL: assetBase + "/images/rabbit.webp"},
						},
						CorrectOptionID: "A",
						Hint:            "Con vật có cái vòi dài bắt đầu bằng chữ E.",
						Explanation:     "Elephant là con voi.",
					},
				},
				{
					OrderIndex: 4, Name: "Chợ trái cây nhiệt đới", ObstacleType: "broken_bridge",
					PosX: 470, PosY: 340, RewardCoins: 70,
					ChallengeTyp: domain.ChallengeTypeMultipleChoice,
					PromptText:   "Quả táo trong tiếng Anh đọc là gì?",
					AudioPath:    "/audio/w3_chl_04.mp3",
					Payload: domain.MultipleChoicePayload{
						Options:         []domain.ChoiceOption{{ID: "A", Text: "Banana"}, {ID: "B", Text: "Apple"}, {ID: "C", Text: "Orange"}},
						CorrectOptionID: "B",
						Hint:            "Bắt đầu bằng chữ cái A!",
						Explanation:     "Apple là quả táo (Banana: chuối, Orange: cam).",
					},
				},
				{
					OrderIndex: 5, Name: "Thuyền ghép từ vựng", ObstacleType: "locked_gate",
					PosX: 590, PosY: 380, RewardCode: "compass_english", RewardCoins: 80,
					ChallengeTyp: domain.ChallengeTypeDragDrop,
					PromptText:   "Kéo từ tiếng Anh tương ứng với nghĩa tiếng Việt.",
					AudioPath:    "/audio/w3_chl_05.mp3",
					Payload: domain.DragDropPayload{
						Items: []domain.DragDropItem{
							{ID: "en_cat", Text: "Cat"},
							{ID: "en_dog", Text: "Dog"},
							{ID: "en_bird", Text: "Bird"},
						},
						Targets: []domain.DragDropTarget{
							{ID: "t1", Label: "Con mèo"},
							{ID: "t2", Label: "Con chó"},
							{ID: "t3", Label: "Con chim"},
						},
						CorrectMap:  map[string]string{"t1": "en_cat", "t2": "en_dog", "t3": "en_bird"},
						Hint:        "Cat kêu meo meo, Dog kêu gâu gâu nhé!",
						Explanation: "Cat = Con mèo, Dog = Con chó, Bird = Con chim.",
					},
				},
				{
					OrderIndex: 6, Name: "Ngôi nhà gia đình", ObstacleType: "sleeping_bear",
					PosX: 700, PosY: 310, RewardCoins: 80,
					ChallengeTyp: domain.ChallengeTypeMultipleChoice,
					PromptText:   "Từ 'Mother' chỉ ai trong gia đình?",
					AudioPath:    "/audio/w3_chl_06.mp3",
					Payload: domain.MultipleChoicePayload{
						Options:         []domain.ChoiceOption{{ID: "A", Text: "Bố"}, {ID: "B", Text: "Mẹ"}, {ID: "C", Text: "Anh trai"}},
						CorrectOptionID: "B",
						Hint:            "Father là Bố, còn Mother là...",
						Explanation:     "Mother nghĩa là Mẹ.",
					},
				},
				{
					OrderIndex: 7, Name: "Cầu số đếm hải âu", ObstacleType: "broken_bridge",
					PosX: 800, PosY: 260, RewardCoins: 90,
					ChallengeTyp: domain.ChallengeTypeMultipleChoice,
					PromptText:   "Số 8 trong tiếng Anh viết như thế nào?",
					AudioPath:    "/audio/w3_chl_07.mp3",
					Payload: domain.MultipleChoicePayload{
						Options:         []domain.ChoiceOption{{ID: "A", Text: "Seven"}, {ID: "B", Text: "Eight"}, {ID: "C", Text: "Nine"}},
						CorrectOptionID: "B",
						Hint:            "Đếm: Six (6), Seven (7), ... (8), Nine (9).",
						Explanation:     "Eight là số 8.",
					},
				},
				{
					OrderIndex: 8, Name: "Cổng lớp học vui vẻ", ObstacleType: "locked_gate",
					PosX: 890, PosY: 300, RewardCoins: 90,
					ChallengeTyp: domain.ChallengeTypeMultipleChoice,
					PromptText:   "Đồ vật nào dùng để viết và có tên là 'Pencil'?",
					AudioPath:    "/audio/w3_chl_08.mp3",
					Payload: domain.MultipleChoicePayload{
						Options:         []domain.ChoiceOption{{ID: "A", Text: "Bút chì"}, {ID: "B", Text: "Cục tẩy"}, {ID: "C", Text: "Thước kẻ"}},
						CorrectOptionID: "A",
						Hint:            "Eraser là cục tẩy, Ruler là thước kẻ.",
						Explanation:     "Pencil là bút chì.",
					},
				},
				{
					OrderIndex: 9, Name: "Ngọn hải đăng thời tiết", ObstacleType: "sleeping_bear",
					PosX: 980, PosY: 220, RewardCoins: 100,
					ChallengeTyp: domain.ChallengeTypeMultipleChoice,
					PromptText:   "Khi trời nắng đẹp, ta nói 'It is ...'?",
					AudioPath:    "/audio/w3_chl_09.mp3",
					Payload: domain.MultipleChoicePayload{
						Options:         []domain.ChoiceOption{{ID: "A", Text: "rainy"}, {ID: "B", Text: "sunny"}, {ID: "C", Text: "snowy"}},
						CorrectOptionID: "B",
						Hint:            "'Sun' nghĩa là mặt trời!",
						Explanation:     "Sunny nghĩa là có nắng.",
					},
				},
				{
					OrderIndex: 10, Name: "Kho Báu Đảo Anh Ngữ", ObstacleType: "locked_gate",
					PosX: 1060, PosY: 140, RewardCode: "island_badge", RewardCoins: 130,
					ChallengeTyp: domain.ChallengeTypeMultipleChoice,
					PromptText:   "Hoàn thành câu: 'Thank you very ...!'",
					AudioPath:    "/audio/w3_chl_10.mp3",
					Payload: domain.MultipleChoicePayload{
						Options:         []domain.ChoiceOption{{ID: "A", Text: "much"}, {ID: "B", Text: "many"}, {ID: "C", Text: "good"}},
						CorrectOptionID: "A",
						Hint:            "Cụm từ cảm ơn rất nhiều quen thuộc trong tiếng Anh.",
						Explanation:     "'Thank you very much!' nghĩa là Cảm ơn bạn rất nhiều!",
					},
				},
			},
		},

		// ============================================================
		// WORLD 4: ĐẠI DƯƠNG KHOA HỌC (Tự nhiên & Khoa học - 10 màn)
		// ============================================================
		{
			World: domain.World{
				Code:        "dai-duong-khoa-hoc",
				Name:        "Đại Dương Khoa Học",
				Description: "Lặn sâu khám phá thế giới tự nhiên, sinh vật biển và những bí ẩn kỳ thú của Trái Đất.",
				OrderIndex:  4,
				MapBgURL:    assetBase + "/maps/dai-duong-khoa-hoc-bg.webp",
				BgmURL:      assetBase + "/audio/bgm-dai-duong-khoa-hoc.mp3",
				IsActive:    true,
			},
			Stages: []stageSeed{
				{
					OrderIndex: 1, Name: "Rạn san hô sắc màu", ObstacleType: "broken_bridge",
					PosX: 150, PosY: 500, RewardCoins: 70,
					ChallengeTyp: domain.ChallengeTypeMultipleChoice,
					PromptText:   "Loài động vật nào dưới đây sống ở dưới nước và thở bằng mang?",
					AudioPath:    "/audio/w4_chl_01.mp3",
					Payload: domain.MultipleChoicePayload{
						Options:         []domain.ChoiceOption{{ID: "A", Text: "Cá hề"}, {ID: "B", Text: "Chim bồ câu"}, {ID: "C", Text: "Thỏ trắng"}},
						CorrectOptionID: "A",
						Hint:            "Loài vật này bơi lội bằng vây dưới biển.",
						Explanation:     "Cá thở bằng mang và sống trong môi trường nước.",
					},
				},
				{
					OrderIndex: 2, Name: "Cổng dòng hải lưu", ObstacleType: "locked_gate",
					PosX: 260, PosY: 430, RewardCoins: 70,
					ChallengeTyp: domain.ChallengeTypeMultipleChoice,
					PromptText:   "Nước đóng băng thành nước đá ở nhiệt độ nào?",
					AudioPath:    "/audio/w4_chl_02.mp3",
					Payload: domain.MultipleChoicePayload{
						Options:         []domain.ChoiceOption{{ID: "A", Text: "0 độ C"}, {ID: "B", Text: "50 độ C"}, {ID: "C", Text: "100 độ C"}},
						CorrectOptionID: "A",
						Hint:            "100 độ C là nhiệt độ nước sôi, còn đóng băng thì rất lạnh!",
						Explanation:     "Nước tinh khiết đóng băng ở 0°C và sôi ở 100°C.",
					},
				},
				{
					OrderIndex: 3, Name: "Chú rùa biển thông thái", ObstacleType: "sleeping_bear",
					PosX: 370, PosY: 380, RewardCode: "diving_helmet", RewardCoins: 80,
					ChallengeTyp: domain.ChallengeTypeMultipleChoice,
					PromptText:   "Cây xanh cần yếu tố nào sau đây để quang hợp tạo ra khí oxy?",
					AudioPath:    "/audio/w4_chl_03.mp3",
					Payload: domain.MultipleChoicePayload{
						Options:         []domain.ChoiceOption{{ID: "A", Text: "Ánh sáng mặt trời"}, {ID: "B", Text: "Bóng tối hoàn toàn"}, {ID: "C", Text: "Sắt và thép"}},
						CorrectOptionID: "A",
						Hint:            "Cây xanh luôn vươn lá về phía Mặt Trời!",
						Explanation:     "Cây xanh sử dụng ánh sáng mặt trời, nước và CO₂ để quang hợp.",
					},
				},
				{
					OrderIndex: 4, Name: "Cầu bong bóng khí", ObstacleType: "broken_bridge",
					PosX: 480, PosY: 330, RewardCoins: 80,
					ChallengeTyp: domain.ChallengeTypeMultipleChoice,
					PromptText:   "Con người có mấy giác quan cơ bản?",
					AudioPath:    "/audio/w4_chl_04.mp3",
					Payload: domain.MultipleChoicePayload{
						Options:         []domain.ChoiceOption{{ID: "A", Text: "3 giác quan"}, {ID: "B", Text: "5 giác quan"}, {ID: "C", Text: "7 giác quan"}},
						CorrectOptionID: "B",
						Hint:            "Thị giác, thính giác, khứu giác, vị giác và xúc giác.",
						Explanation:     "Con người có 5 giác quan cơ bản.",
					},
				},
				{
					OrderIndex: 5, Name: "Phòng thí nghiệm dưới biển", ObstacleType: "locked_gate",
					PosX: 590, PosY: 370, RewardCode: "ocean_suit", RewardCoins: 90,
					ChallengeTyp: domain.ChallengeTypeDragDrop,
					PromptText:   "Ghép từng bộ phận của cây với chức năng chính của nó.",
					AudioPath:    "/audio/w4_chl_05.mp3",
					Payload: domain.DragDropPayload{
						Items: []domain.DragDropItem{
							{ID: "part_root", Text: "Rễ cây"},
							{ID: "part_leaf", Text: "Lá cây"},
							{ID: "part_stem", Text: "Thân cây"},
						},
						Targets: []domain.DragDropTarget{
							{ID: "t1", Label: "Hút nước và muối khoáng từ đất"},
							{ID: "t2", Label: "Quang hợp hấp thụ ánh sáng"},
							{ID: "t3", Label: "Nâng đỡ cành lá và vận chuyển nước"},
						},
						CorrectMap:  map[string]string{"t1": "part_root", "t2": "part_leaf", "t3": "part_stem"},
						Hint:        "Rễ nằm dưới đất, lá màu xanh đón nắng nhé!",
						Explanation: "Rễ hút nước, thân vận chuyển và nâng đỡ, lá quang hợp.",
					},
				},
				{
					OrderIndex: 6, Name: "Vườn sứa phát quang", ObstacleType: "sleeping_bear",
					PosX: 700, PosY: 300, RewardCoins: 90,
					ChallengeTyp: domain.ChallengeTypeMultipleChoice,
					PromptText:   "Hành tinh nào được gọi là 'Hành tinh Xanh' và là ngôi nhà của chúng ta?",
					AudioPath:    "/audio/w4_chl_06.mp3",
					Payload: domain.MultipleChoicePayload{
						Options:         []domain.ChoiceOption{{ID: "A", Text: "Sao Hỏa"}, {ID: "B", Text: "Trái Đất"}, {ID: "C", Text: "Sao Thổ"}},
						CorrectOptionID: "B",
						Hint:            "Phần lớn bề mặt hành tinh này được bao phủ bởi đại dương xanh.",
						Explanation:     "Trái Đất được mệnh danh là Hành tinh Xanh.",
					},
				},
				{
					OrderIndex: 7, Name: "Cầu vồng hơi nước", ObstacleType: "broken_bridge",
					PosX: 800, PosY: 250, RewardCoins: 90,
					ChallengeTyp: domain.ChallengeTypeMultipleChoice,
					PromptText:   "Mưa được tạo thành từ đâu?",
					AudioPath:    "/audio/w4_chl_07.mp3",
					Payload: domain.MultipleChoicePayload{
						Options:         []domain.ChoiceOption{{ID: "A", Text: "Hơi nước ngưng tụ thành mây rồi rơi xuống"}, {ID: "B", Text: "Từ cát trên sa mạc"}, {ID: "C", Text: "Từ đá núi lửa"}},
						CorrectOptionID: "A",
						Hint:            "Nước bốc hơi lên trời tạo thành những đám mây nặng hạt.",
						Explanation:     "Nước bốc hơi, ngưng tụ thành mây và rơi xuống tạo thành mưa.",
					},
				},
				{
					OrderIndex: 8, Name: "Cánh cổng cá heo", ObstacleType: "locked_gate",
					PosX: 890, PosY: 290, RewardCoins: 100,
					ChallengeTyp: domain.ChallengeTypeMultipleChoice,
					PromptText:   "Mặt Trời mọc ở hướng nào mỗi buổi sáng?",
					AudioPath:    "/audio/w4_chl_08.mp3",
					Payload: domain.MultipleChoicePayload{
						Options:         []domain.ChoiceOption{{ID: "A", Text: "Hướng Đông"}, {ID: "B", Text: "Hướng Tây"}, {ID: "C", Text: "Hướng Bắc"}},
						CorrectOptionID: "A",
						Hint:            "Mặt Trời mọc ở đằng Đông và lặn ở đằng Tây.",
						Explanation:     "Mặt Trời mọc ở hướng Đông.",
					},
				},
				{
					OrderIndex: 9, Name: "Bạch tuộc bảo vệ biển", ObstacleType: "sleeping_bear",
					PosX: 980, PosY: 210, RewardCoins: 100,
					ChallengeTyp: domain.ChallengeTypeMultipleChoice,
					PromptText:   "Hành động nào giúp bảo vệ đại dương sạch đẹp?",
					AudioPath:    "/audio/w4_chl_09.mp3",
					Payload: domain.MultipleChoicePayload{
						Options:         []domain.ChoiceOption{{ID: "A", Text: "Vứt chai nhựa xuống biển"}, {ID: "B", Text: "Bỏ rác đúng nơi quy định và hạn chế túi nilon"}, {ID: "C", Text: "Bẻ san hô mang về"}},
						CorrectOptionID: "B",
						Hint:            "Rác thải nhựa rất có hại cho sinh vật biển.",
						Explanation:     "Bỏ rác đúng nơi quy định giúp bảo vệ sinh vật và môi trường biển.",
					},
				},
				{
					OrderIndex: 10, Name: "Thủy Cung Khoa Học", ObstacleType: "locked_gate",
					PosX: 1060, PosY: 140, RewardCode: "science_badge", RewardCoins: 140,
					ChallengeTyp: domain.ChallengeTypeMultipleChoice,
					PromptText:   "Thử thách cuối! Loài động vật lớn nhất hiện nay sống dưới đại dương là gì?",
					AudioPath:    "/audio/w4_chl_10.mp3",
					Payload: domain.MultipleChoicePayload{
						Options:         []domain.ChoiceOption{{ID: "A", Text: "Cá voi xanh"}, {ID: "B", Text: "Cá mập trắng"}, {ID: "C", Text: "Bạch tuộc khổng lồ"}},
						CorrectOptionID: "A",
						Hint:            "Đây là loài thú biển khổng lồ có màu xanh xám.",
						Explanation:     "Cá voi xanh là loài động vật lớn nhất trên Trái Đất!",
					},
				},
			},
		},

		// ============================================================
		// WORLD 5: VƯƠNG QUỐC TƯ DUY (Logic & Giải đố - 10 màn)
		// ============================================================
		{
			World: domain.World{
				Code:        "vuong-quoc-tu-duy",
				Name:        "Vương Quốc Tư Duy",
				Description: "Rèn luyện trí thông minh qua các câu đố logic, quy luật và tư duy sáng tạo.",
				OrderIndex:  5,
				MapBgURL:    assetBase + "/maps/vuong-quoc-tu-duy-bg.webp",
				BgmURL:      assetBase + "/audio/bgm-vuong-quoc-tu-duy.mp3",
				IsActive:    true,
			},
			Stages: []stageSeed{
				{
					OrderIndex: 1, Name: "Cầu quy luật dãy số", ObstacleType: "broken_bridge",
					PosX: 150, PosY: 510, RewardCoins: 80,
					ChallengeTyp: domain.ChallengeTypeMultipleChoice,
					PromptText:   "Tìm số tiếp theo của dãy: 2, 4, 6, 8, ...",
					AudioPath:    "/audio/w5_chl_01.mp3",
					Payload: domain.MultipleChoicePayload{
						Options:         []domain.ChoiceOption{{ID: "A", Text: "9"}, {ID: "B", Text: "10"}, {ID: "C", Text: "12"}},
						CorrectOptionID: "B",
						Hint:            "Mỗi số sau lớn hơn số trước 2 đơn vị.",
						Explanation:     "8 + 2 = 10.",
					},
				},
				{
					OrderIndex: 2, Name: "Cổng tìm điểm khác biệt", ObstacleType: "locked_gate",
					PosX: 260, PosY: 440, RewardCoins: 80,
					ChallengeTyp: domain.ChallengeTypeMultipleChoice,
					PromptText:   "Đồ vật nào KHÔNG cùng nhóm với các vật còn lại?",
					AudioPath:    "/audio/w5_chl_02.mp3",
					Payload: domain.MultipleChoicePayload{
						Options:         []domain.ChoiceOption{{ID: "A", Text: "Quả cam"}, {ID: "B", Text: "Quả chuối"}, {ID: "C", Text: "Chiếc xe đạp"}},
						CorrectOptionID: "C",
						Hint:            "Hai thứ là trái cây, thứ còn lại là phương tiện giao thông.",
						Explanation:     "Chiếc xe đạp không phải là trái cây.",
					},
				},
				{
					OrderIndex: 3, Name: "Cú mèo đố vui", ObstacleType: "sleeping_bear",
					PosX: 370, PosY: 380, RewardCode: "logic_spectacles", RewardCoins: 90,
					ChallengeTyp: domain.ChallengeTypeMultipleChoice,
					PromptText:   "Hôm nay là Thứ Ba. Hỏi ngày mai là Thứ mấy?",
					AudioPath:    "/audio/w5_chl_03.mp3",
					Payload: domain.MultipleChoicePayload{
						Options:         []domain.ChoiceOption{{ID: "A", Text: "Thứ Hai"}, {ID: "B", Text: "Thứ Tư"}, {ID: "C", Text: "Thứ Năm"}},
						CorrectOptionID: "B",
						Hint:            "Đếm tiếp sau Thứ Ba nhé!",
						Explanation:     "Sau Thứ Ba là Thứ Tư.",
					},
				},
				{
					OrderIndex: 4, Name: "Cầu bậc thang số lẻ", ObstacleType: "broken_bridge",
					PosX: 480, PosY: 330, RewardCoins: 90,
					ChallengeTyp: domain.ChallengeTypeMultipleChoice,
					PromptText:   "Tìm số tiếp theo: 5, 10, 15, 20, ...",
					AudioPath:    "/audio/w5_chl_04.mp3",
					Payload: domain.MultipleChoicePayload{
						Options:         []domain.ChoiceOption{{ID: "A", Text: "22"}, {ID: "B", Text: "25"}, {ID: "C", Text: "30"}},
						CorrectOptionID: "B",
						Hint:            "Mỗi bước cộng thêm 5 đơn vị.",
						Explanation:     "20 + 5 = 25.",
					},
				},
				{
					OrderIndex: 5, Name: "Tháp phân loại thông minh", ObstacleType: "locked_gate",
					PosX: 590, PosY: 370, RewardCode: "royal_armor", RewardCoins: 100,
					ChallengeTyp: domain.ChallengeTypeDragDrop,
					PromptText:   "Kéo từng phương tiện vào môi trường di chuyển đúng.",
					AudioPath:    "/audio/w5_chl_05.mp3",
					Payload: domain.DragDropPayload{
						Items: []domain.DragDropItem{
							{ID: "v_plane", Text: "Máy bay"},
							{ID: "v_ship", Text: "Tàu thủy"},
							{ID: "v_car", Text: "Ô tô"},
						},
						Targets: []domain.DragDropTarget{
							{ID: "t1", Label: "Trên bầu trời"},
							{ID: "t2", Label: "Trên mặt nước"},
							{ID: "t3", Label: "Trên đường bộ"},
						},
						CorrectMap:  map[string]string{"t1": "v_plane", "t2": "v_ship", "t3": "v_car"},
						Hint:        "Máy bay bay trên trời, tàu thủy chạy dưới nước!",
						Explanation: "Máy bay (trời), Tàu thủy (nước), Ô tô (đường bộ).",
					},
				},
				{
					OrderIndex: 6, Name: "Vườn hoa quy luật màu", ObstacleType: "sleeping_bear",
					PosX: 700, PosY: 300, RewardCoins: 100,
					ChallengeTyp: domain.ChallengeTypeMultipleChoice,
					PromptText:   "Quan sát dãy màu: Đỏ - Xanh - Vàng - Đỏ - Xanh - ... Màu tiếp theo là gì?",
					AudioPath:    "/audio/w5_chl_06.mp3",
					Payload: domain.MultipleChoicePayload{
						Options:         []domain.ChoiceOption{{ID: "A", Text: "Đỏ"}, {ID: "B", Text: "Xanh"}, {ID: "C", Text: "Vàng"}},
						CorrectOptionID: "C",
						Hint:            "Bộ 3 màu lặp lại theo thứ tự Đỏ → Xanh → Vàng.",
						Explanation:     "Sau Xanh là Vàng.",
					},
				},
				{
					OrderIndex: 7, Name: "Cầu cân bằng suy luận", ObstacleType: "broken_bridge",
					PosX: 800, PosY: 250, RewardCoins: 110,
					ChallengeTyp: domain.ChallengeTypeMultipleChoice,
					PromptText:   "An cao hơn Bình, Bình cao hơn Chi. Hỏi ai là người cao nhất?",
					AudioPath:    "/audio/w5_chl_07.mp3",
					Payload: domain.MultipleChoicePayload{
						Options:         []domain.ChoiceOption{{ID: "A", Text: "An"}, {ID: "B", Text: "Bình"}, {ID: "C", Text: "Chi"}},
						CorrectOptionID: "A",
						Hint:            "Xếp theo thứ tự từ cao đến thấp: An > Bình > Chi.",
						Explanation:     "An cao hơn cả Bình và Chi nên An cao nhất.",
					},
				},
				{
					OrderIndex: 8, Name: "Cổng mật mã đảo ngược", ObstacleType: "locked_gate",
					PosX: 890, PosY: 290, RewardCoins: 110,
					ChallengeTyp: domain.ChallengeTypeMultipleChoice,
					PromptText:   "Tìm số còn thiếu trong dãy giảm dần: 30, 27, 24, 21, ...",
					AudioPath:    "/audio/w5_chl_08.mp3",
					Payload: domain.MultipleChoicePayload{
						Options:         []domain.ChoiceOption{{ID: "A", Text: "19"}, {ID: "B", Text: "18"}, {ID: "C", Text: "17"}},
						CorrectOptionID: "B",
						Hint:            "Mỗi số sau giảm đi 3 đơn vị so với số trước.",
						Explanation:     "21 - 3 = 18.",
					},
				},
				{
					OrderIndex: 9, Name: "Rồng con giữ cửa", ObstacleType: "sleeping_bear",
					PosX: 980, PosY: 210, RewardCoins: 120,
					ChallengeTyp: domain.ChallengeTypeMultipleChoice,
					PromptText:   "Một con vịt đi trước 2 con vịt, 1 con vịt đi sau 2 con vịt, 1 con vịt đi giữa 2 con vịt. Hỏi có ít nhất mấy con vịt?",
					AudioPath:    "/audio/w5_chl_09.mp3",
					Payload: domain.MultipleChoicePayload{
						Options:         []domain.ChoiceOption{{ID: "A", Text: "3 con vịt"}, {ID: "B", Text: "5 con vịt"}, {ID: "C", Text: "6 con vịt"}},
						CorrectOptionID: "A",
						Hint:            "Hãy tưởng tượng các con vịt xếp thành một hàng dọc!",
						Explanation:     "Chỉ cần 3 con vịt đi nối đuôi nhau theo một hàng dọc.",
					},
				},
				{
					OrderIndex: 10, Name: "Cung Điện Bậc Thầy Tư Duy", ObstacleType: "locked_gate",
					PosX: 1070, PosY: 140, RewardCode: "mastermind_crown", RewardCoins: 150,
					ChallengeTyp: domain.ChallengeTypeMultipleChoice,
					PromptText:   "Thử thách tối thượng! Nếu 1 cái bánh cắt làm đôi được 2 miếng, rồi cắt đôi mỗi miếng một lần nữa thì được bao nhiêu miếng bánh?",
					AudioPath:    "/audio/w5_chl_10.mp3",
					Payload: domain.MultipleChoicePayload{
						Options:         []domain.ChoiceOption{{ID: "A", Text: "3 miếng"}, {ID: "B", Text: "4 miếng"}, {ID: "C", Text: "6 miếng"}},
						CorrectOptionID: "B",
						Hint:            "Mỗi miếng trong 2 miếng đầu tiên lại chia thành 2 miếng nhỏ.",
						Explanation:     "2 x 2 = 4 miếng bánh. Chúc mừng bé đã hoàn thành Vương Quốc Tư Duy!",
					},
				},
			},
		},
	}
}

func ensureUser(tx *gorm.DB, email, password, role string) (domain.User, error) {
	var user domain.User
	err := tx.Where("email = ?", email).First(&user).Error
	if err == nil {
		return user, nil
	}
	if err != gorm.ErrRecordNotFound {
		return user, err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return user, err
	}
	user = domain.User{
		Email:        email,
		PasswordHash: string(hash),
		Role:         role,
	}
	return user, tx.Create(&user).Error
}

func ensureProfile(tx *gorm.DB, userID uuid.UUID, nickname, avatarURL string) (domain.Profile, error) {
	var profile domain.Profile
	err := tx.Where("user_id = ? AND nickname = ?", userID, nickname).First(&profile).Error
	if err == nil {
		return profile, nil
	}
	if err != gorm.ErrRecordNotFound {
		return profile, err
	}
	profile = domain.Profile{
		UserID:    userID,
		Nickname:  nickname,
		AvatarURL: avatarURL,
	}
	return profile, tx.Create(&profile).Error
}

func ensureItem(tx *gorm.DB, item domain.Item) (domain.Item, error) {
	var existing domain.Item
	err := tx.Where("code = ?", item.Code).First(&existing).Error
	if err == nil {
		return existing, nil
	}
	if err != gorm.ErrRecordNotFound {
		return existing, err
	}
	return item, tx.Create(&item).Error
}

func ensureWorld(tx *gorm.DB, world domain.World) (domain.World, error) {
	var existing domain.World
	err := tx.Where("code = ?", world.Code).First(&existing).Error
	if err == nil {
		return existing, nil
	}
	if err != gorm.ErrRecordNotFound {
		return existing, err
	}
	return world, tx.Create(&world).Error
}

func ensureStage(tx *gorm.DB, stage domain.Stage) (domain.Stage, error) {
	var existing domain.Stage
	err := tx.Where("world_id = ? AND order_index = ?", stage.WorldID, stage.OrderIndex).First(&existing).Error
	if err == nil {
		return existing, nil
	}
	if err != gorm.ErrRecordNotFound {
		return existing, err
	}
	return stage, tx.Create(&stage).Error
}

func ensureChallenge(tx *gorm.DB, stageID uuid.UUID, orderIdx int, chType, promptText, audioURL string, payloadObj any) error {
	var count int64
	if err := tx.Model(&domain.Challenge{}).
		Where("stage_id = ? AND order_index = ?", stageID, orderIdx).
		Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	raw, err := json.Marshal(payloadObj)
	if err != nil {
		return err
	}
	ch := domain.Challenge{
		StageID:        stageID,
		Type:           chType,
		PromptText:     promptText,
		PromptAudioURL: audioURL,
		Payload:        datatypes.JSON(raw),
		OrderIndex:     orderIdx,
	}
	return tx.Create(&ch).Error
}
