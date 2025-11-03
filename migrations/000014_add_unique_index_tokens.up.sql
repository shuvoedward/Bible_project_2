-- authentication token does not have unique constrant
-- allows users to be signed in multiple device
CREATE UNIQUE INDEX idx_tokens_user_scope_verification
ON tokens(user_id, scope)
WHERE scope IN ('activation', 'password-reset');