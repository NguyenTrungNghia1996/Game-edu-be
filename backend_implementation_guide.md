# TÀI LIỆU KỸ THUẬT & HƯỚNG DẪN TRIỂN KHAI BACKEND (BE)

**Dự án:** Game Giáo Dục Phiêu Lưu Đa Nền Tảng (VD: *Rừng Chữ Cái*)  
**Tech Stack:** Golang (1.23+), GoFiber v2, GORM, PostgreSQL 16 (JSONB), Redis 7, Cloudflare R2 (AWS SDK v2), Docker.

---

## 1. Kiến trúc tổng quan Backend

Backend đóng vai trò là **Single Source of Truth** cho toàn bộ tiến độ học tập, cơ chế chống gian lận đáp án, quản lý nội dung màn chơi (CMS) và cấp quyền upload tài nguyên đa phương tiện lên Cloudflare R2.

```text
[Client: Web / Android / iOS]
          │
          │ HTTPS REST API (JWT + X-Profile-ID)
          ▼
[Reverse Proxy: Nginx / Nginx Proxy Manager]
          │
          ▼
[GoFiber API Server] ──► [Redis 7] (Cache Map/World, Attempt Session, Rate Limit)
          │
          ├──► [PostgreSQL 16] (Users, Profiles, Worlds, Stages, Challenges JSONB, Progress)
          │
          └──► [Cloudflare R2] (Presigned PUT URL cho Admin; Public CDN URL cho Client)
```

---

## 2. Cấu trúc thư mục (Clean Architecture)

```text
edu-game-be/
├── cmd/
│   ├── api/
│   │   └── main.go                 # Entrypoint khởi chạy Fiber server & Dependency Injection
│   └── seed/
│       └── main.go                 # CLI nạp dữ liệu mẫu (Rừng Chữ Cái - 10 màn chơi)
├── internal/
│   ├── config/
│   │   └── config.go               # Load & validate biến môi trường (.env)
│   ├── database/
│   │   ├── postgres.go             # Khởi tạo kết nối GORM + Connection Pool
│   │   └── redis.go                # Khởi tạo Redis client
│   ├── storage/
│   │   └── r2_client.go            # Kết nối Cloudflare R2 qua AWS SDK v2 (Presigned URL)
│   ├── domain/                     # Entities (GORM Models) & DTOs
│   │   ├── user.go
│   │   ├── profile.go
│   │   ├── world.go
│   │   ├── stage.go
│   │   ├── challenge.go            # Chứa struct JSONB Payload cho từng dạng câu hỏi
│   │   ├── item.go
│   │   └── progress.go
│   ├── repository/                 # Tầng truy vấn Database (GORM)
│   │   ├── auth_repo.go
│   │   ├── world_repo.go
│   │   ├── stage_repo.go
│   │   └── progress_repo.go
│   ├── service/                    # Tầng xử lý nghiệp vụ cốt lõi
│   │   ├── auth_service.go
│   │   ├── gameplay_service.go     # Chấm điểm, Transaction mở khóa màn/vùng, thưởng vật phẩm
│   │   ├── inventory_service.go
│   │   └── admin_service.go
│   ├── handler/                    # Tầng HTTP Controllers (GoFiber)
│   │   ├── auth_handler.go
│   │   ├── profile_handler.go
│   │   ├── gameplay_handler.go
│   │   ├── inventory_handler.go
│   │   └── admin_handler.go
│   └── middleware/
│       ├── auth.go                 # Xác thực JWT & kiểm tra Role (parent / admin)
│       ├── profile_context.go      # Validate header X-Profile-ID thuộc sở hữu user hiện tại
│       └── cors.go                 # Cấu hình CORS cho Web + Capacitor Mobile
├── migrations/                     # File SQL Migration (golang-migrate)
│   ├── 000001_init_schema.up.sql
│   └── 000001_init_schema.down.sql
├── Dockerfile
├── docker-compose.yml
├── go.mod
└── .env.example
```

---

## 3. Thiết kế Cơ sở dữ liệu (PostgreSQL 16 + GORM)

### 3.1. SQL Migration khởi tạo (`migrations/000001_init_schema.up.sql`)

```sql
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- 1. Tài khoản đăng nhập (Phụ huynh / Admin)
CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email VARCHAR(255) UNIQUE NOT NULL,
    password_hash TEXT NOT NULL,
    role VARCHAR(20) NOT NULL DEFAULT 'parent', -- 'parent' | 'admin'
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- 2. Hồ sơ người chơi (1 phụ huynh có thể tạo nhiều hồ sơ cho các bé)
CREATE TABLE profiles (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    nickname VARCHAR(100) NOT NULL,
    avatar_url TEXT,
    coins INT NOT NULL DEFAULT 0,
    total_stars INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_profiles_user_id ON profiles(user_id);

-- 3. Vật phẩm phần thưởng
CREATE TABLE items (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    code VARCHAR(64) UNIQUE NOT NULL,
    name VARCHAR(150) NOT NULL,
    description TEXT,
    category VARCHAR(32) NOT NULL, -- 'hat' | 'outfit' | 'badge' | 'tool'
    icon_url TEXT NOT NULL,
    sprite_url TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- 4. Vùng đất (World)
CREATE TABLE worlds (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    code VARCHAR(64) UNIQUE NOT NULL,       -- VD: 'rung-chu-cai'
    name VARCHAR(150) NOT NULL,             -- VD: 'Rừng Chữ Cái'
    description TEXT,
    order_index INT UNIQUE NOT NULL,        -- 1, 2, 3... xác định thứ tự mở khóa
    map_bg_url TEXT NOT NULL,
    bgm_url TEXT,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- 5. Màn chơi (10 - 15 màn / vùng đất)
CREATE TABLE stages (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    world_id UUID NOT NULL REFERENCES worlds(id) ON DELETE CASCADE,
    order_index INT NOT NULL,               -- 1 -> 15
    name VARCHAR(150) NOT NULL,             -- VD: 'Cây cầu gỗ bị gãy'
    obstacle_type VARCHAR(50) NOT NULL,     -- 'broken_bridge' | 'locked_gate' | 'sleeping_bear'
    pos_x INT NOT NULL DEFAULT 0,           -- Tọa độ X trên bản đồ vùng đất
    pos_y INT NOT NULL DEFAULT 0,           -- Tọa độ Y trên bản đồ vùng đất
    reward_item_id UUID REFERENCES items(id) ON DELETE SET NULL,
    reward_coins INT NOT NULL DEFAULT 50,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (world_id, order_index)
);
CREATE INDEX idx_stages_world_id ON stages(world_id);

-- 6. Thử thách / Câu hỏi (Lưu cấu trúc linh hoạt bằng JSONB)
CREATE TABLE challenges (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    stage_id UUID NOT NULL REFERENCES stages(id) ON DELETE CASCADE,
    type VARCHAR(32) NOT NULL,              -- 'multiple_choice' | 'drag_drop' | 'audio_match'
    prompt_text TEXT NOT NULL,
    prompt_audio_url TEXT,
    payload JSONB NOT NULL,
    order_index INT NOT NULL DEFAULT 1,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_challenges_stage_id ON challenges(stage_id);

-- 7. Tiến độ từng màn của bé
CREATE TABLE profile_progress (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    profile_id UUID NOT NULL REFERENCES profiles(id) ON DELETE CASCADE,
    stage_id UUID NOT NULL REFERENCES stages(id) ON DELETE CASCADE,
    status VARCHAR(20) NOT NULL DEFAULT 'locked', -- 'locked' | 'unlocked' | 'completed'
    stars INT NOT NULL DEFAULT 0,                 -- 0..3
    best_wrong_attempts INT NOT NULL DEFAULT 0,
    completed_at TIMESTAMPTZ,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (profile_id, stage_id)
);
CREATE INDEX idx_progress_profile_status ON profile_progress(profile_id, status);

-- 8. Túi đồ (Inventory) của bé
CREATE TABLE profile_inventory (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    profile_id UUID NOT NULL REFERENCES profiles(id) ON DELETE CASCADE,
    item_id UUID NOT NULL REFERENCES items(id) ON DELETE CASCADE,
    quantity INT NOT NULL DEFAULT 1,
    is_equipped BOOLEAN NOT NULL DEFAULT FALSE,
    acquired_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (profile_id, item_id)
);
```

### 3.2. Cấu trúc JSONB Payload (`internal/domain/challenge.go`)

```go
package domain

import (
	"github.com/google/uuid"
	"gorm.io/datatypes"
)

type Challenge struct {
	ID             uuid.UUID      `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	StageID        uuid.UUID      `gorm:"type:uuid;not null;index" json:"stage_id"`
	Type           string         `gorm:"type:varchar(32);not null" json:"type"`
	PromptText     string         `gorm:"type:text;not null" json:"prompt_text"`
	PromptAudioURL string         `gorm:"type:text" json:"prompt_audio_url"`
	Payload        datatypes.JSON `gorm:"type:jsonb;not null" json:"payload"`
	OrderIndex     int            `gorm:"not null;default:1" json:"order_index"`
}

// Payload lưu trong DB cho dạng trắc nghiệm (multiple_choice)
type MultipleChoicePayload struct {
	Options         []ChoiceOption `json:"options"`
	CorrectOptionID string         `json:"correct_option_id"` // TUYỆT ĐỐI KHÔNG TRẢ VỀ KHI START STAGE
	Hint            string         `json:"hint,omitempty"`
	Explanation     string         `json:"explanation,omitempty"`
}

type ChoiceOption struct {
	ID       string `json:"id"`
	Text     string `json:"text"`
	AudioURL string `json:"audio_url,omitempty"`
	ImageURL string `json:"image_url,omitempty"`
}
```

---

## 4. Đặc tả API (RESTful Specification)

**Base URL:** `/api/v1`  
**Auth Headers:**
* `Authorization: Bearer <access_token>`
* `X-Profile-ID: <profile_uuid>` (bắt buộc cho các API nhóm Gameplay & Inventory)

### 4.1. Danh sách Endpoints

| Nhóm | Method | Endpoint | Mô tả |
| :--- | :--- | :--- | :--- |
| **Auth** | `POST` | `/auth/register` | Đăng ký tài khoản phụ huynh |
| | `POST` | `/auth/login` | Đăng nhập, nhận `access_token` & `refresh_token` |
| **Profile** | `GET` | `/profiles` | Lấy danh sách hồ sơ bé của phụ huynh |
| | `POST` | `/profiles` | Tạo hồ sơ bé mới (tự động mở khóa World 1 - Stage 1) |
| **World** | `GET` | `/worlds` | Lấy danh sách vùng đất kèm tiến độ (`completed_stages / total_stages`) |
| | `GET` | `/worlds/:worldId/map` | Lấy chi tiết bản đồ + 10–15 màn chơi kèm trạng thái (`locked/unlocked/completed`) |
| **Gameplay** | `POST` | `/stages/:stageId/start` | Bắt đầu thử thách tại chướng ngại vật, nhận `attempt_id` & câu hỏi (đã ẩn đáp án) |
| | `POST` | `/stages/:stageId/submit` | Gửi đáp án, chấm điểm, mở khóa màn/vùng mới và trả thưởng |
| **Inventory** | `GET` | `/inventory` | Danh sách vật phẩm trong túi đồ |
| | `PATCH` | `/inventory/:itemId/equip` | Trang bị / tháo vật phẩm cho nhân vật |
| **Admin** | `POST` | `/admin/upload-url` | Lấy Presigned PUT URL để upload asset thẳng lên Cloudflare R2 |
| | `CRUD` | `/admin/{worlds,stages,challenges,items}` | Quản lý toàn bộ nội dung game |

### 4.2. Chi tiết Payload cho 2 API trọng yếu

**1. `POST /api/v1/stages/:stageId/start`**
*Response `200 OK`:*
```json
{
  "attempt_id": "att_9f8a7c6b5d4e",
  "stage": {
    "id": "stg_01",
    "name": "Cây cầu gỗ bị gãy",
    "obstacle_type": "broken_bridge"
  },
  "challenges": [
    {
      "id": "chl_01",
      "type": "multiple_choice",
      "prompt_text": "Chọn từ viết đúng chính tả.",
      "prompt_audio_url": "https://assets.yourdomain.com/audio/chl_01_prompt.mp3",
      "options": [
        { "id": "A", "text": "Chân thành", "audio_url": "https://assets.yourdomain.com/audio/opt_a.mp3" },
        { "id": "B", "text": "Trân thành", "audio_url": "https://assets.yourdomain.com/audio/opt_b.mp3" },
        { "id": "C", "text": "Chân thàng", "audio_url": "https://assets.yourdomain.com/audio/opt_c.mp3" }
      ]
    }
  ]
}
```

**2. `POST /api/v1/stages/:stageId/submit`**
*Request:*
```json
{
  "attempt_id": "att_9f8a7c6b5d4e",
  "challenge_id": "chl_01",
  "answer": "A"
}
```
*Response khi trả lời ĐÚNG (`200 OK`):*
```json
{
  "is_correct": true,
  "explanation": "Chân thành (viết bằng Ch) có nghĩa là thật lòng.",
  "stage_completed": true,
  "stars_earned": 3,
  "first_time_clear": true,
  "rewards": {
    "coins": 50,
    "item": {
      "id": "itm_wooden_hammer",
      "name": "Búa Gỗ Thần Kỳ",
      "icon_url": "https://assets.yourdomain.com/items/wooden_hammer.webp"
    }
  },
  "unlocks": {
    "next_stage_id": "stg_02",
    "next_world_id": null
  }
}
```
*Response khi trả lời SAI (`200 OK`):*
```json
{
  "is_correct": false,
  "hint": "Hãy lắng nghe âm đầu 'Ch' và vần 'anh' nhé!",
  "wrong_attempts": 1,
  "stage_completed": false
}
```

---

## 5. Xử lý nghiệp vụ cốt lõi (Business Logic)

### 5.1. Cơ chế Session & Chống gian lận bằng Redis
* Khi gọi `POST /stages/:stageId/start`:
  1. Kiểm tra `profile_progress`: bắt buộc trạng thái phải là `unlocked` hoặc `completed` (nếu `locked` trả về `403 Forbidden`).
  2. Lưu key Redis `attempt:{attempt_id}` dạng Hash gồm `{profile_id, stage_id, wrong_count: 0}` với TTL **30 phút**.
  3. Loại bỏ hoàn toàn `correct_option_id` khỏi JSON trước khi trả cho Client.
* Khi trả lời sai: Tăng `wrong_count` trong Redis bằng `HINCRBY` để tính số sao khi hoàn thành (`0 sai = 3 sao`, `1 sai = 2 sao`, `>= 2 sai = 1 sao`).

### 5.2. Transaction hoàn thành màn & Mở khóa vùng đất mới
Bọc toàn bộ logic khi trẻ trả lời đúng câu cuối của màn trong `db.Transaction`:

1. **Row-level Lock:** Dùng `clause.Locking{Strength: "UPDATE"}` khi đọc bản ghi `profile_progress` của `(profile_id, stage_id)` để ngăn lỗi gửi trùng request (double submit).
2. **Kiểm tra nhận thưởng lần đầu:**
   * `isFirstClear := progress.Status != "completed"`
   * Nếu `isFirstClear`: Cộng `reward_coins` vào bảng `profiles`, đồng thời `INSERT ... ON CONFLICT DO UPDATE` cộng vật phẩm `reward_item_id` vào `profile_inventory`.
   * Nếu chơi lại: Chỉ cập nhật `stars` nếu số sao mới lớn hơn số sao cũ, không phát lại vật phẩm.
3. **Mở khóa màn tiếp theo:**
   * Tìm `stage` cùng `world_id` có `order_index = currentStage.OrderIndex + 1`.
   * Nếu có -> tạo hoặc giữ nguyên trạng thái `unlocked` trong `profile_progress`.
4. **Mở khóa vùng đất tiếp theo (khi hoàn thành màn cuối của vùng):**
   * Nếu không còn màn kế tiếp trong vùng hiện tại -> tìm `world` có `order_index = currentWorld.OrderIndex + 1` và `is_active = true`.
   * Tìm `stage` đầu tiên (`order_index = 1`) của vùng mới đó -> mở khóa `unlocked` trong `profile_progress` và trả `next_world_id` về Client.
5. Xóa key `attempt:{attempt_id}` trên Redis.

### 5.3. Tích hợp Cloudflare R2 (`internal/storage/r2_client.go`)

Sử dụng `github.com/aws/aws-sdk-go-v2/service/s3`:

```go
package storage

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type R2Storage struct {
	client        *s3.Client
	presignClient *s3.PresignClient
	bucketName    string
	publicURL     string
}

func NewR2Storage(accountID, accessKey, secretKey, bucket, publicURL string) (*R2Storage, error) {
	endpoint := fmt.Sprintf("https://%s.r2.cloudflarestorage.com", accountID)

	cfg, err := config.LoadDefaultConfig(context.Background(),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(accessKey, secretKey, "")),
		config.WithRegion("auto"),
	)
	if err != nil {
		return nil, err
	}

	client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(endpoint)
	})

	return &R2Storage{
		client:        client,
		presignClient: s3.NewPresignClient(client),
		bucketName:    bucket,
		publicURL:     publicURL,
	}, nil
}

// GenerateUploadURL tạo Presigned PUT URL (hết hạn 5 phút) cho Admin CMS upload trực tiếp
func (r *R2Storage) GenerateUploadURL(ctx context.Context, objectKey, contentType string) (uploadURL, publicFileURL string, err error) {
	req, err := r.presignClient.PresignPutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(r.bucketName),
		Key:         aws.String(objectKey),
		ContentType: aws.String(contentType),
	}, s3.WithPresignExpires(5*time.Minute))
	if err != nil {
		return "", "", err
	}

	return req.URL, fmt.Sprintf("%s/%s", r.publicURL, objectKey), nil
}
```

**Cấu hình CORS bắt buộc trên Cloudflare R2 Dashboard** (R2 -> Bucket -> Settings -> CORS Policy) để Phaser WebGL tải được ảnh và Admin upload được file:
```json
[
  {
    "AllowedOrigins": [
      "https://game.yourdomain.com",
      "https://admin.yourdomain.com",
      "capacitor://localhost",
      "https://localhost",
      "http://localhost:3000"
    ],
    "AllowedMethods": ["GET", "HEAD", "PUT"],
    "AllowedHeaders": ["*"],
    "ExposeHeaders": ["ETag"],
    "MaxAgeSeconds": 86400
  }
]
```

---

## 6. Hướng dẫn triển khai (Local & Production)

### 6.1. File cấu hình môi trường (`.env.example`)

```env
APP_ENV=production
APP_PORT=8080
JWT_SECRET=thay_bang_chuoi_bi_mat_toi_thieu_32_ky_tu
JWT_EXPIRE_HOURS=168

# PostgreSQL
DB_HOST=postgres
DB_PORT=5432
DB_USER=edugame
DB_PASSWORD=StrongDbPassword123!
DB_NAME=edugame_db
DB_SSLMODE=disable

# Redis
REDIS_ADDR=redis:6379
REDIS_PASSWORD=StrongRedisPassword123!
REDIS_DB=0

# Cloudflare R2
R2_ACCOUNT_ID=your_account_id
R2_ACCESS_KEY_ID=your_access_key
R2_SECRET_ACCESS_KEY=your_secret_key
R2_BUCKET_NAME=edugame-assets
R2_PUBLIC_URL=https://assets.yourdomain.com

# CORS (Bắt buộc có capacitor://localhost cho iOS và https://localhost cho Android)
CORS_ALLOWED_ORIGINS=https://game.yourdomain.com,https://admin.yourdomain.com,capacitor://localhost,https://localhost,http://localhost:3000
```

### 6.2. Multi-stage `Dockerfile`

```dockerfile
# Stage 1: Build binary
FROM golang:1.23-alpine AS builder
WORKDIR /app

RUN apk add --no-cache git ca-certificates

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /app/bin/api ./cmd/api
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /app/bin/seed ./cmd/seed

# Stage 2: Runtime gọn nhẹ
FROM alpine:3.20
RUN apk --no-cache add ca-certificates tzdata
ENV TZ=Asia/Ho_Chi_Minh

WORKDIR /app
COPY --from=builder /app/bin/api .
COPY --from=builder /app/bin/seed .
COPY --from=builder /app/migrations ./migrations

EXPOSE 8080
CMD ["./api"]
```

### 6.3. File `docker-compose.yml`

```yaml
services:
  postgres:
    image: postgres:16-alpine
    container_name: edugame-postgres
    restart: unless-stopped
    environment:
      POSTGRES_USER: ${DB_USER}
      POSTGRES_PASSWORD: ${DB_PASSWORD}
      POSTGRES_DB: ${DB_NAME}
    volumes:
      - pg_data:/var/lib/postgresql/data
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U ${DB_USER} -d ${DB_NAME}"]
      interval: 10s
      timeout: 5s
      retries: 5

  redis:
    image: redis:7-alpine
    container_name: edugame-redis
    restart: unless-stopped
    command: redis-server --appendonly yes --requirepass ${REDIS_PASSWORD}
    volumes:
      - redis_data:/data
    healthcheck:
      test: ["CMD", "redis-cli", "-a", "${REDIS_PASSWORD}", "ping"]
      interval: 10s
      timeout: 5s
      retries: 5

  api:
    build: .
    container_name: edugame-api
    restart: unless-stopped
    env_file: .env
    ports:
      - "127.0.0.1:8080:8080"
    depends_on:
      postgres:
        condition: service_healthy
      redis:
        condition: service_healthy

volumes:
  pg_data:
  redis_data:
```

### 6.4. Các bước triển khai lên Server Production

1. **Khởi tạo thư mục & biến môi trường:**
   ```bash
   git clone <repo-be-url> edu-game-be && cd edu-game-be
   cp .env.example .env
   nano .env # Điền thông tin thực tế
   ```
2. **Build & khởi chạy container:**
   ```bash
   docker compose up -d --build
   ```
3. **Chạy Migration & Seed dữ liệu Rừng Chữ Cái:**
   ```bash
   # Chạy migrate SQL (hoặc tích hợp auto-migrate khi khởi động)
   docker compose exec api ./seed
   ```
4. **Cấu hình Reverse Proxy (Nginx / Nginx Proxy Manager):**
   * Tạo Proxy Host cho `api.yourdomain.com` trỏ về `http://127.0.0.1:8080`.
   * Bật **Force SSL** (Let's Encrypt) và **HTTP/2**.
   * Đặt `client_max_body_size 10M;` (asset lớn đã upload trực tiếp lên R2 qua Presigned URL nên API chỉ nhận JSON).

### 6.5. Checklist kiểm thử Backend trước khi bàn giao FE
- [ ] `GET /health` trả về `200 OK` (kiểm tra ping Postgres & Redis).
- [ ] `POST /stages/:id/start` tuyệt đối không lộ `correct_option_id`.
- [ ] Gửi đồng thời 2 request `POST /stages/:id/submit` với cùng đáp án đúng -> chỉ cộng xu & vật phẩm đúng **1 lần**.
- [ ] Hoàn thành màn cuối (VD: màn 10/10) của Rừng Chữ Cái -> tự động mở khóa màn 1 của Vùng Đất số 2.
- [ ] Gọi API từ origin `capacitor://localhost` và `https://localhost` không bị chặn CORS.