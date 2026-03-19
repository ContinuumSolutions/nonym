#!/bin/bash

# Comment out the session existence check in ValidateToken function
# This makes the authentication rely solely on JWT validation
sed -i '/Check if session exists and is valid/,/session expired or invalid/ {
s/^/\/\/ TEMPORARILY DISABLED: /
}' /workspaces/EK-1/pkg/auth/auth.go

# Also comment out the createSession call error handling in LoginUser
sed -i '/Create session/,/Failed to create session/ {
/if err != nil/{
N
N
s/if err != nil {/\/\/ if err != nil {/
s/log.Printf/\/\/ log.Printf/
s/}/\/\/ }/
}
}' /workspaces/EK-1/pkg/auth/auth.go

echo "Temporarily disabled session validation - using JWT-only authentication"
