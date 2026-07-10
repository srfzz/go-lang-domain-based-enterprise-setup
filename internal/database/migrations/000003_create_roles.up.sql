CREATE TABLE  IF NOT EXISTS roles (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(100) UNIQUE NOT NULL,
    category VARCHAR(100) NOT NULL DEFAULT 'general',
    description TEXT,
    created_at TIMESTAMPTZ DEFAULT now()
);
