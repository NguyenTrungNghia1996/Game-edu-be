CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- 1. Tài khoản đăng nhập (Phụ huynh / Admin)
CREATE TABLE IF NOT EXISTS users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email VARCHAR(255) UNIQUE NOT NULL,
    password_hash TEXT NOT NULL,
    role VARCHAR(20) NOT NULL DEFAULT 'parent', -- 'parent' | 'admin'
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- 2. Hồ sơ người chơi (1 phụ huynh có thể tạo nhiều hồ sơ cho các bé)
CREATE TABLE IF NOT EXISTS profiles (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    nickname VARCHAR(100) NOT NULL,
    avatar_url TEXT,
    coins INT NOT NULL DEFAULT 0,
    total_stars INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_profiles_user_id ON profiles(user_id);

-- 3. Vật phẩm phần thưởng
CREATE TABLE IF NOT EXISTS items (
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
CREATE TABLE IF NOT EXISTS worlds (
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
CREATE TABLE IF NOT EXISTS stages (
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
CREATE INDEX IF NOT EXISTS idx_stages_world_id ON stages(world_id);

-- 6. Thử thách / Câu hỏi (Lưu cấu trúc linh hoạt bằng JSONB)
CREATE TABLE IF NOT EXISTS challenges (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    stage_id UUID NOT NULL REFERENCES stages(id) ON DELETE CASCADE,
    type VARCHAR(32) NOT NULL,              -- 'multiple_choice' | 'drag_drop' | 'audio_match'
    prompt_text TEXT NOT NULL,
    prompt_audio_url TEXT,
    payload JSONB NOT NULL,
    order_index INT NOT NULL DEFAULT 1,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_challenges_stage_id ON challenges(stage_id);

-- 7. Tiến độ từng màn của bé
CREATE TABLE IF NOT EXISTS profile_progress (
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
CREATE INDEX IF NOT EXISTS idx_progress_profile_status ON profile_progress(profile_id, status);

-- 8. Túi đồ (Inventory) của bé
CREATE TABLE IF NOT EXISTS profile_inventory (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    profile_id UUID NOT NULL REFERENCES profiles(id) ON DELETE CASCADE,
    item_id UUID NOT NULL REFERENCES items(id) ON DELETE CASCADE,
    quantity INT NOT NULL DEFAULT 1,
    is_equipped BOOLEAN NOT NULL DEFAULT FALSE,
    acquired_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (profile_id, item_id)
);
