#!/bin/bash

# Create a temporary file with the correct auth routes
cat > /tmp/auth_routes.go << 'EOF'
	// Authentication routes (must come BEFORE /api/* proxy route)
	authGroup := app.Group("/api/auth")
	authGroup.Post("/login", auth.HandleLogin)
	authGroup.Post("/register", auth.HandleRegister)
	authGroup.Post("/logout", auth.HandleLogout)
	authGroup.Get("/me", auth.AuthMiddleware, auth.HandleGetMe)
EOF

# Remove the corrupted auth section (lines 155-180)
sed -i '155,180d' /workspaces/EK-1/cmd/gateway/main.go

# Insert the new auth section at line 155
sed -i '154r /tmp/auth_routes.go' /workspaces/EK-1/cmd/gateway/main.go

# Clean up
rm /tmp/auth_routes.go

echo "Auth section fixed successfully"
