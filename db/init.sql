-- Example schema for your HFT engine
CREATE TABLE IF NOT EXISTS orders (
    id UUID PRIMARY KEY,
    user_id VARCHAR(255),
    ticker VARCHAR(10),
    side VARCHAR(4),
    price BIGINT,
    quantity BIGINT,
    status VARCHAR(20),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);