-- Add role column to users table
ALTER TABLE users
ADD COLUMN role VARCHAR(20) NOT NULL DEFAULT 'user';

-- Create index on role for faster filtering
CREATE INDEX idx_users_role ON users(role);

-- Add check constraint to ensure valid roles
ALTER TABLE users
ADD CONSTRAINT check_users_role CHECK (role IN ('user', 'admin'));

COMMENT ON COLUMN users.role IS 'User role: user or admin';
