#!/bin/bash

# Add crypto/sha256 import
sed -i '/import (/a\\t"crypto/sha256"' /workspaces/EK-1/pkg/auth/auth.go

# Replace the bcrypt session creation with SHA256
sed -i 's/\/\/ Hash the token for storage/\/\/ Hash the token for storage using SHA256/' /workspaces/EK-1/pkg/auth/auth.go

# Replace the actual bcrypt call
sed -i 's/tokenHash, err := hashPassword(token)/hash := sha256.Sum256([]byte(token))\
\ttokenHash := fmt.Sprintf("%x", hash)\
\terr := error(nil)/' /workspaces/EK-1/pkg/auth/auth.go

# Fix the session validation query in ValidateToken
sed -i 's/SELECT EXISTS(SELECT 1 FROM user_sessions WHERE user_id = ? AND expires_at > CURRENT_TIMESTAMP)/SELECT EXISTS(SELECT 1 FROM user_sessions WHERE user_id = ? AND token = ? AND expires_at > CURRENT_TIMESTAMP)/' /workspaces/EK-1/pkg/auth/auth.go

# Update the query execution to include token hash
sed -i 's/err = db.QueryRow(".*user_sessions.*expires_at.*").Scan(&sessionExists)/hash := sha256.Sum256([]byte(tokenString))\
\ttokenHash := fmt.Sprintf("%x", hash)\
\terr = db.QueryRow("SELECT EXISTS(SELECT 1 FROM user_sessions WHERE user_id = ? AND token = ? AND expires_at > CURRENT_TIMESTAMP)", userID, tokenHash).Scan(\&sessionExists)/' /workspaces/EK-1/pkg/auth/auth.go

echo "Session storage fixed to use SHA256 instead of bcrypt"
