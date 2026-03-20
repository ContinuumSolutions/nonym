# Sidebar Menu Features Implementation

## Overview
This document describes the comprehensive sidebar menu implementation for the Nonym dashboard, including new views, API endpoints, and security features.

## Frontend Implementation

### New Components
1. **Sidebar.vue** - Main navigation sidebar component with role-based menu items
2. **DashboardLayout.vue** - Layout wrapper that includes the sidebar for all authenticated views
3. **ProtectedEvents.vue** - Detailed protection events viewer with filtering and pagination
4. **Integrations.vue** - API key management and AI provider configuration
5. **Account.vue** - Organization settings, team management, billing, and security

### Updated Components
- **Dashboard.vue** - Modified to use the new DashboardLayout wrapper
- **main.js** - Updated router with new routes for sidebar menu items

### New Routes
- `/protected-events` - Protection events detail view
- `/integrations` - API keys and AI provider management
- `/account` - Organization and team management

### Enhanced API Service
Added endpoints for:
- API key management (`/api-keys`)
- Provider configuration (`/provider-config`, `/providers/:provider/test`)
- Organization management (`/organization`)
- Team management (`/team/members`)
- Security settings (`/security/*`)

## Backend Implementation

### New Database Tables
```sql
-- API Keys for programmatic access
CREATE TABLE api_keys (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    key_hash TEXT NOT NULL,
    masked_key TEXT NOT NULL,
    permissions TEXT NOT NULL,
    user_id TEXT NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    expires_at DATETIME,
    status TEXT DEFAULT 'active',
    last_used DATETIME
);

-- Encrypted provider configurations
CREATE TABLE provider_configs (
    user_id TEXT PRIMARY KEY,
    config_data TEXT NOT NULL,  -- AES-GCM encrypted
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Organization management
CREATE TABLE organizations (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    industry TEXT,
    size TEXT,
    country TEXT,
    description TEXT,
    owner_id INTEGER NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (owner_id) REFERENCES users (id)
);
```

### New API Handlers

#### API Key Management
- `GET /api/v1/api-keys` - List user's API keys
- `POST /api/v1/api-keys` - Create new API key
- `PATCH /api/v1/api-keys/:id/revoke` - Revoke API key
- `DELETE /api/v1/api-keys/:id` - Delete API key

#### Provider Configuration
- `GET /api/v1/provider-config` - Get AI provider settings
- `PUT /api/v1/provider-config` - Save provider configuration
- `POST /api/v1/providers/:provider/test` - Test provider connection

#### Organization Management
- `GET /api/v1/organization` - Get organization details
- `PUT /api/v1/organization` - Update organization
- `GET /api/v1/team/members` - List team members
- `POST /api/v1/team/members` - Invite team member
- `DELETE /api/v1/team/members/:id` - Remove team member

#### Security Features
- `PUT /api/v1/security/2fa` - Update two-factor authentication
- `DELETE /api/v1/security/sessions/:id` - Terminate user session
- `PUT /api/v1/security/settings` - Update security settings

### Security Features

#### API Key Encryption
- API keys are hashed using bcrypt before storage
- Only masked versions are displayed in the UI
- Full keys are only shown once during creation

#### Provider Configuration Encryption
- API keys for AI providers are encrypted using AES-GCM
- Unique encryption key derived per user using PBKDF2
- Configuration stored encrypted in database

#### Authentication & Authorization
- JWT-based session management
- API key middleware for programmatic access
- Role-based access control (owner, admin, member, viewer)
- Protected routes require valid authentication

## Feature Highlights

### 1. Dashboard
- Real-time privacy protection metrics
- Recent activity feed
- Protection impact visualization

### 2. Protected Events
- Detailed event filtering by time, type, and status
- Pagination for large datasets
- Event detail modal with redaction information
- Real-time statistics dashboard

### 3. Integrations
#### API Key Management
- Generate keys with custom names and permissions
- Set expiration dates for enhanced security
- Revoke or delete keys as needed
- Copy keys to clipboard functionality

#### AI Provider Configuration
- Support for OpenAI, Anthropic, Google, and local LLMs
- Connection testing with real API validation
- Model selection and configuration
- Encrypted storage of sensitive credentials

### 4. Account Management
#### Organization Settings
- Company details and industry information
- Compliance settings (GDPR, HIPAA, CCPA, SOC 2)
- Company size and location tracking

#### Team Management
- Invite members with role-based permissions
- View team member activity and status
- Remove or suspend team members
- Track invitation and join dates

#### Billing Information
- Current plan details and usage metrics
- Usage tracking for requests, data, and team size
- Payment method management

#### Security Settings
- Two-factor authentication toggle
- Active session management
- API access control settings
- IP whitelisting and request signing options

## Usage Examples

### Creating an API Key
1. Navigate to Integrations → API Keys tab
2. Enter key name and select permissions (read/write/admin)
3. Optionally set expiration date
4. Click "Generate Key" to create
5. Copy the generated key (shown only once)

### Configuring AI Providers
1. Go to Integrations → AI Engines tab
2. Enter API key for desired provider (OpenAI, Anthropic, etc.)
3. Select which models to enable
4. Click "Test Connection" to validate
5. Save configuration to encrypt and store

### Managing Team Members
1. Access Account → Team Members tab
2. Enter email and role for new member
3. Click "Send Invite" to invite
4. View member status and activity
5. Use actions to edit roles or remove members

## Security Considerations

- API keys use secure random generation and bcrypt hashing
- Provider credentials are encrypted with AES-GCM using user-specific keys
- All sensitive operations require authentication
- Rate limiting and request validation on all endpoints
- Audit logging for security events
- Session management with JWT tokens
- Optional 2FA for enhanced account security

## Future Enhancements

- Real-time WebSocket updates for protection events
- Advanced analytics and reporting
- Custom privacy policies and rules
- Integration with external SIEM systems
- Multi-organization support
- Advanced role-based permissions
- Compliance audit trails and reports
