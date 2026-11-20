package main

import (
	"encoding/json"
	"log"
	"strings"

	"edu-game-be/internal/config"
	"edu-game-be/internal/database"
	"edu-game-be/internal/domain"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/datatypes"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

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

	log.Println("Seed completed successfully!")
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
		itemDefs := []domain.Item{
			{
				Code:        "wooden_hammer",
				Name:        "Búa Gỗ Thần Kỳ",
				Description: "Dụng cụ sửa chữa cây cầu gỗ trong Rừng Chữ Cái.",
				Category:    domain.ItemCategoryTool,
				IconURL:     assetBase + "/items/wooden_hammer.webp",
				SpriteURL:   assetBase + "/sprites/wooden_hammer.png",
			},
			{
				Code:        "explorer_hat",
				Name:        "Mũ Thám Hiểm Rừng Xanh",
				Description: "Chiếc mũ dành cho nhà thám hiểm nhí gan dạ.",
				Category:    domain.ItemCategoryHat,
				IconURL:     assetBase + "/items/explorer_hat.webp",
				SpriteURL:   assetBase + "/sprites/explorer_hat.png",
			},
			{
				Code:        "forest_cape",
				Name:        "Áo Choàng Lá Xanh",
				Description: "Áo choàng phép thuật giúp băng qua khu rừng rậm.",
				Category:    domain.ItemCategoryOutfit,
				IconURL:     assetBase + "/items/forest_cape.webp",
				SpriteURL:   assetBase + "/sprites/forest_cape.png",
			},
			{
				Code:        "spelling_badge",
				Name:        "Huy Hiệu Chính Tả Vàng",
				Description: "Phần thưởng cho bé vượt qua thử thách chính tả khó nhằn.",
				Category:    domain.ItemCategoryBadge,
				IconURL:     assetBase + "/items/spelling_badge.webp",
				SpriteURL:   assetBase + "/sprites/spelling_badge.png",
			},
			{
				Code:        "magic_lantern",
				Name:        "Đèn Lồng Đom Đóm",
				Description: "Soi sáng con đường xuyên qua Hang Gấu Ngủ.",
				Category:    domain.ItemCategoryTool,
				IconURL:     assetBase + "/items/magic_lantern.webp",
				SpriteURL:   assetBase + "/sprites/magic_lantern.png",
			},
			{
				Code:        "golden_crown",
				Name:        "Vương Miện Rừng Chữ Cái",
				Description: "Biểu tượng chiến thắng toàn bộ vùng đất Rừng Chữ Cái.",
				Category:    domain.ItemCategoryHat,
				IconURL:     assetBase + "/items/golden_crown.webp",
				SpriteURL:   assetBase + "/sprites/golden_crown.png",
			},
		}

		itemsByCode := make(map[string]domain.Item, len(itemDefs))
		for _, item := range itemDefs {
			saved, err := ensureItem(tx, item)
			if err != nil {
				return err
			}
			itemsByCode[saved.Code] = saved
		}

		// 3. Seed World 1: Rừng Chữ Cái (10 màn chơi)
		world1, err := ensureWorld(tx, domain.World{
			Code:        "rung-chu-cai",
			Name:        "Rừng Chữ Cái",
			Description: "Hành trình chinh phục chính tả tiếng Việt và giải cứu khu rừng kỳ diệu.",
			OrderIndex:  1,
			MapBgURL:    assetBase + "/maps/rung-chu-cai-bg.webp",
			BgmURL:      assetBase + "/audio/bgm-rung-chu-cai.mp3",
			IsActive:    true,
		})
		if err != nil {
			return err
		}

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

		world1Stages := []stageSeed{
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
					CorrectMap: map[string]string{
						"t1": "item_r",
						"t2": "item_gi",
						"t3": "item_d",
					},
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
		}

		var firstStageWorld1ID uuid.UUID
		for _, sd := range world1Stages {
			var rewardItemID *uuid.UUID
			if sd.RewardCode != "" {
				item := itemsByCode[sd.RewardCode]
				id := item.ID
				rewardItemID = &id
			}
			stage, err := ensureStage(tx, domain.Stage{
				WorldID:      world1.ID,
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
			if sd.OrderIndex == 1 {
				firstStageWorld1ID = stage.ID
			}
			if err := ensureChallenge(tx, stage.ID, 1, sd.ChallengeTyp, sd.PromptText, assetBase+sd.AudioPath, sd.Payload); err != nil {
				return err
			}
		}

		// 4. Seed World 2: Thung Lũng Con Số (để kiểm thử mở khóa vùng mới sau màn 10/10)
		world2, err := ensureWorld(tx, domain.World{
			Code:        "thung-lung-con-so",
			Name:        "Thung Lũng Con Số",
			Description: "Khám phá phép tính vui nhộn cùng những người bạn số học.",
			OrderIndex:  2,
			MapBgURL:    assetBase + "/maps/thung-lung-con-so-bg.webp",
			BgmURL:      assetBase + "/audio/bgm-thung-lung-con-so.mp3",
			IsActive:    true,
		})
		if err != nil {
			return err
		}

		world2Stages := []stageSeed{
			{
				OrderIndex: 1, Name: "Cánh cổng số học", ObstacleType: "locked_gate",
				PosX: 160, PosY: 480, RewardCoins: 60,
				ChallengeTyp: domain.ChallengeTypeMultipleChoice,
				PromptText:   "Tính nhanh: 7 + 5 bằng bao nhiêu?",
				AudioPath:    "/audio/w2_chl_01.mp3",
				Payload: domain.MultipleChoicePayload{
					Options: []domain.ChoiceOption{
						{ID: "A", Text: "11"},
						{ID: "B", Text: "12"},
						{ID: "C", Text: "13"},
					},
					CorrectOptionID: "B",
					Hint:            "7 + 3 = 10, thêm 2 nữa là bao nhiêu nhỉ?",
					Explanation:     "7 + 5 = 12.",
				},
			},
			{
				OrderIndex: 2, Name: "Cầu đá phép trừ", ObstacleType: "broken_bridge",
				PosX: 340, PosY: 390, RewardCoins: 60,
				ChallengeTyp: domain.ChallengeTypeMultipleChoice,
				PromptText:   "Tính nhanh: 15 - 6 bằng bao nhiêu?",
				AudioPath:    "/audio/w2_chl_02.mp3",
				Payload: domain.MultipleChoicePayload{
					Options: []domain.ChoiceOption{
						{ID: "A", Text: "8"},
						{ID: "B", Text: "9"},
						{ID: "C", Text: "10"},
					},
					CorrectOptionID: "B",
					Hint:            "15 - 5 = 10, bớt thêm 1 nữa nhé!",
					Explanation:     "15 - 6 = 9.",
				},
			},
			{
				OrderIndex: 3, Name: "Chú gấu đếm táo", ObstacleType: "sleeping_bear",
				PosX: 520, PosY: 310, RewardCoins: 80,
				ChallengeTyp: domain.ChallengeTypeMultipleChoice,
				PromptText:   "Mỗi giỏ có 3 quả táo. Hỏi 4 giỏ có tất cả bao nhiêu quả táo?",
				AudioPath:    "/audio/w2_chl_03.mp3",
				Payload: domain.MultipleChoicePayload{
					Options: []domain.ChoiceOption{
						{ID: "A", Text: "10 quả"},
						{ID: "B", Text: "12 quả"},
						{ID: "C", Text: "14 quả"},
					},
					CorrectOptionID: "B",
					Hint:            "3 + 3 + 3 + 3 = ?",
					Explanation:     "3 x 4 = 12 quả táo.",
				},
			},
		}

		for _, sd := range world2Stages {
			stage, err := ensureStage(tx, domain.Stage{
				WorldID:      world2.ID,
				OrderIndex:   sd.OrderIndex,
				Name:         sd.Name,
				ObstacleType: sd.ObstacleType,
				PosX:         sd.PosX,
				PosY:         sd.PosY,
				RewardCoins:  sd.RewardCoins,
			})
			if err != nil {
				return err
			}
			if err := ensureChallenge(tx, stage.ID, 1, sd.ChallengeTyp, sd.PromptText, assetBase+sd.AudioPath, sd.Payload); err != nil {
				return err
			}
		}

		// 5. Seed demo Profile for parent user & unlock World 1 - Stage 1
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
