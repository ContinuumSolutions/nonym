<template>
  <DashboardLayout>
    <div class="p-8">
      <!-- Header -->
      <div class="mb-8">
        <h1 class="text-3xl font-bold text-gray-900">Account Settings</h1>
        <p class="text-gray-600 mt-2">Manage your organization settings and team members</p>
      </div>

      <!-- Tabs -->
      <div class="mb-6">
        <nav class="flex space-x-8">
          <button
            @click="activeTab = 'organization'"
            :class="[
              'py-2 px-1 border-b-2 font-medium text-sm',
              activeTab === 'organization'
                ? 'border-blue-500 text-blue-600'
                : 'border-transparent text-gray-500 hover:text-gray-700 hover:border-gray-300'
            ]">
            Organization
          </button>
          <button
            @click="activeTab = 'team'"
            :class="[
              'py-2 px-1 border-b-2 font-medium text-sm',
              activeTab === 'team'
                ? 'border-blue-500 text-blue-600'
                : 'border-transparent text-gray-500 hover:text-gray-700 hover:border-gray-300'
            ]">
            Team Members
          </button>
          <button
            @click="activeTab = 'billing'"
            :class="[
              'py-2 px-1 border-b-2 font-medium text-sm',
              activeTab === 'billing'
                ? 'border-blue-500 text-blue-600'
                : 'border-transparent text-gray-500 hover:text-gray-700 hover:border-gray-300'
            ]">
            Billing
          </button>
          <button
            @click="activeTab = 'security'"
            :class="[
              'py-2 px-1 border-b-2 font-medium text-sm',
              activeTab === 'security'
                ? 'border-blue-500 text-blue-600'
                : 'border-transparent text-gray-500 hover:text-gray-700 hover:border-gray-300'
            ]">
            Security
          </button>
        </nav>
      </div>

      <!-- Organization Tab -->
      <div v-if="activeTab === 'organization'" class="space-y-6">
        <!-- Organization Details -->
        <div class="bg-white rounded-lg shadow-sm p-6">
          <h3 class="text-lg font-medium text-gray-900 mb-4">Organization Details</h3>
          <div class="grid grid-cols-1 md:grid-cols-2 gap-6">
            <div>
              <label class="block text-sm font-medium text-gray-700 mb-2">Organization Name</label>
              <input
                v-model="organization.name"
                type="text"
                class="w-full px-3 py-2 border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500">
            </div>
            <div>
              <label class="block text-sm font-medium text-gray-700 mb-2">Industry</label>
              <select v-model="organization.industry" class="w-full px-3 py-2 border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500">
                <option value="">Select Industry</option>
                <option value="technology">Technology</option>
                <option value="healthcare">Healthcare</option>
                <option value="finance">Finance</option>
                <option value="education">Education</option>
                <option value="legal">Legal</option>
                <option value="other">Other</option>
              </select>
            </div>
            <div>
              <label class="block text-sm font-medium text-gray-700 mb-2">Company Size</label>
              <select v-model="organization.size" class="w-full px-3 py-2 border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500">
                <option value="">Select Size</option>
                <option value="1-10">1-10 employees</option>
                <option value="11-50">11-50 employees</option>
                <option value="51-200">51-200 employees</option>
                <option value="201-1000">201-1,000 employees</option>
                <option value="1000+">1,000+ employees</option>
              </select>
            </div>
            <div>
              <label class="block text-sm font-medium text-gray-700 mb-2">Country</label>
              <select v-model="organization.country" class="w-full px-3 py-2 border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500">
                <option value="">Select Country</option>
                <option value="US">United States</option>
                <option value="CA">Canada</option>
                <option value="GB">United Kingdom</option>
                <option value="DE">Germany</option>
                <option value="FR">France</option>
                <option value="AU">Australia</option>
                <option value="other">Other</option>
              </select>
            </div>
          </div>
          <div class="mt-4">
            <label class="block text-sm font-medium text-gray-700 mb-2">Description</label>
            <textarea
              v-model="organization.description"
              rows="3"
              class="w-full px-3 py-2 border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500"
              placeholder="Brief description of your organization..."></textarea>
          </div>
        </div>

        <!-- Compliance Settings -->
        <div class="bg-white rounded-lg shadow-sm p-6">
          <h3 class="text-lg font-medium text-gray-900 mb-4">Compliance Settings</h3>
          <div class="space-y-4">
            <div class="flex items-start">
              <div class="flex items-center h-5">
                <input v-model="organization.compliance.gdpr" type="checkbox" class="rounded border-gray-300 text-blue-600 focus:ring-blue-500">
              </div>
              <div class="ml-3">
                <label class="text-sm font-medium text-gray-700">GDPR Compliance</label>
                <p class="text-sm text-gray-500">Enable GDPR-specific data protection features</p>
              </div>
            </div>
            <div class="flex items-start">
              <div class="flex items-center h-5">
                <input v-model="organization.compliance.hipaa" type="checkbox" class="rounded border-gray-300 text-blue-600 focus:ring-blue-500">
              </div>
              <div class="ml-3">
                <label class="text-sm font-medium text-gray-700">HIPAA Compliance</label>
                <p class="text-sm text-gray-500">Enable healthcare-specific privacy protections</p>
              </div>
            </div>
            <div class="flex items-start">
              <div class="flex items-center h-5">
                <input v-model="organization.compliance.ccpa" type="checkbox" class="rounded border-gray-300 text-blue-600 focus:ring-blue-500">
              </div>
              <div class="ml-3">
                <label class="text-sm font-medium text-gray-700">CCPA Compliance</label>
                <p class="text-sm text-gray-500">Enable California Consumer Privacy Act protections</p>
              </div>
            </div>
            <div class="flex items-start">
              <div class="flex items-center h-5">
                <input v-model="organization.compliance.soc2" type="checkbox" class="rounded border-gray-300 text-blue-600 focus:ring-blue-500">
              </div>
              <div class="ml-3">
                <label class="text-sm font-medium text-gray-700">SOC 2 Type II</label>
                <p class="text-sm text-gray-500">Enable SOC 2 compliance monitoring and reporting</p>
              </div>
            </div>
          </div>
        </div>

        <!-- Save Button -->
        <div class="flex justify-end">
          <button
            @click="saveOrganization"
            class="px-6 py-2 bg-blue-600 text-white rounded-md hover:bg-blue-700 transition-colors duration-200">
            Save Changes
          </button>
        </div>
      </div>

      <!-- Team Members Tab -->
      <div v-if="activeTab === 'team'" class="space-y-6">
        <!-- Invite Member -->
        <div class="bg-white rounded-lg shadow-sm p-6">
          <h3 class="text-lg font-medium text-gray-900 mb-4">Invite Team Member</h3>
          <div class="grid grid-cols-1 md:grid-cols-3 gap-4">
            <div>
              <input
                v-model="newMember.email"
                type="email"
                placeholder="Email address"
                class="w-full px-3 py-2 border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500">
            </div>
            <div>
              <select v-model="newMember.role" class="w-full px-3 py-2 border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500">
                <option value="member">Member</option>
                <option value="admin">Admin</option>
                <option value="viewer">Viewer</option>
              </select>
            </div>
            <div>
              <button
                @click="inviteMember"
                :disabled="!newMember.email"
                class="w-full px-4 py-2 bg-blue-600 text-white rounded-md hover:bg-blue-700 disabled:opacity-50 transition-colors duration-200">
                Send Invite
              </button>
            </div>
          </div>
        </div>

        <!-- Team Members List -->
        <div class="bg-white rounded-lg shadow-sm overflow-hidden">
          <div class="px-6 py-4 border-b border-gray-200">
            <h3 class="text-lg font-medium text-gray-900">Team Members</h3>
          </div>
          <div class="overflow-x-auto">
            <table class="min-w-full divide-y divide-gray-200">
              <thead class="bg-gray-50">
                <tr>
                  <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Member</th>
                  <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Role</th>
                  <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Status</th>
                  <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Joined</th>
                  <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Last Active</th>
                  <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Actions</th>
                </tr>
              </thead>
              <tbody class="bg-white divide-y divide-gray-200">
                <tr v-for="member in teamMembers" :key="member.id" class="hover:bg-gray-50">
                  <td class="px-6 py-4 whitespace-nowrap">
                    <div class="flex items-center">
                      <div class="w-8 h-8 bg-gray-300 rounded-full flex items-center justify-center">
                        <span class="text-sm font-medium text-gray-600">{{ member.name.charAt(0) }}</span>
                      </div>
                      <div class="ml-4">
                        <div class="text-sm font-medium text-gray-900">{{ member.name }}</div>
                        <div class="text-sm text-gray-500">{{ member.email }}</div>
                      </div>
                    </div>
                  </td>
                  <td class="px-6 py-4 whitespace-nowrap">
                    <span class="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium"
                          :class="getRoleColor(member.role)">
                      {{ member.role }}
                    </span>
                  </td>
                  <td class="px-6 py-4 whitespace-nowrap">
                    <span class="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium"
                          :class="getStatusColor(member.status)">
                      {{ member.status }}
                    </span>
                  </td>
                  <td class="px-6 py-4 whitespace-nowrap text-sm text-gray-500">
                    {{ formatDate(member.joined_at) }}
                  </td>
                  <td class="px-6 py-4 whitespace-nowrap text-sm text-gray-500">
                    {{ member.last_active ? formatDate(member.last_active) : 'Never' }}
                  </td>
                  <td class="px-6 py-4 whitespace-nowrap text-sm font-medium space-x-2">
                    <button
                      v-if="member.status === 'active' && member.role !== 'owner'"
                      @click="updateMemberRole(member)"
                      class="text-blue-600 hover:text-blue-900">
                      Edit
                    </button>
                    <button
                      v-if="member.role !== 'owner'"
                      @click="removeMember(member.id)"
                      class="text-red-600 hover:text-red-900">
                      Remove
                    </button>
                  </td>
                </tr>
              </tbody>
            </table>
          </div>
        </div>
      </div>

      <!-- Billing Tab -->
      <div v-if="activeTab === 'billing'" class="space-y-6">
        <!-- Current Plan -->
        <div class="bg-white rounded-lg shadow-sm p-6">
          <h3 class="text-lg font-medium text-gray-900 mb-4">Current Plan</h3>
          <div class="grid grid-cols-1 md:grid-cols-2 gap-6">
            <div>
              <div class="flex items-center mb-2">
                <span class="text-2xl font-bold text-gray-900">{{ billing.plan.name }}</span>
                <span class="ml-2 inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-green-100 text-green-800">
                  Active
                </span>
              </div>
              <p class="text-sm text-gray-500">{{ billing.plan.description }}</p>
              <div class="mt-4">
                <span class="text-lg font-semibold text-gray-900">${{ billing.plan.price }}/month</span>
                <p class="text-sm text-gray-500">Billed monthly</p>
              </div>
            </div>
            <div>
              <h4 class="font-medium text-gray-900 mb-2">Plan Includes:</h4>
              <ul class="space-y-1">
                <li v-for="feature in billing.plan.features" :key="feature" class="flex items-center text-sm text-gray-600">
                  <svg class="w-4 h-4 mr-2 text-green-500" fill="currentColor" viewBox="0 0 20 20">
                    <path fill-rule="evenodd" d="M16.707 5.293a1 1 0 010 1.414l-8 8a1 1 0 01-1.414 0l-4-4a1 1 0 011.414-1.414L8 12.586l7.293-7.293a1 1 0 011.414 0z" clip-rule="evenodd"/>
                  </svg>
                  {{ feature }}
                </li>
              </ul>
            </div>
          </div>
        </div>

        <!-- Usage -->
        <div class="bg-white rounded-lg shadow-sm p-6">
          <h3 class="text-lg font-medium text-gray-900 mb-4">Current Usage</h3>
          <div class="grid grid-cols-1 md:grid-cols-3 gap-6">
            <div>
              <div class="text-sm font-medium text-gray-700">Requests This Month</div>
              <div class="text-2xl font-bold text-gray-900">{{ billing.usage.requests.toLocaleString() }}</div>
              <div class="w-full bg-gray-200 rounded-full h-2 mt-2">
                <div class="bg-blue-600 h-2 rounded-full" :style="{ width: billing.usage.requestsPercent + '%' }"></div>
              </div>
              <div class="text-xs text-gray-500 mt-1">{{ billing.usage.requestsPercent }}% of {{ billing.usage.requestsLimit.toLocaleString() }} limit</div>
            </div>
            <div>
              <div class="text-sm font-medium text-gray-700">Data Processed</div>
              <div class="text-2xl font-bold text-gray-900">{{ billing.usage.dataGB }}GB</div>
              <div class="w-full bg-gray-200 rounded-full h-2 mt-2">
                <div class="bg-green-600 h-2 rounded-full" :style="{ width: billing.usage.dataPercent + '%' }"></div>
              </div>
              <div class="text-xs text-gray-500 mt-1">{{ billing.usage.dataPercent }}% of {{ billing.usage.dataLimit }}GB limit</div>
            </div>
            <div>
              <div class="text-sm font-medium text-gray-700">Team Members</div>
              <div class="text-2xl font-bold text-gray-900">{{ teamMembers.filter(m => m.status === 'active').length }}</div>
              <div class="w-full bg-gray-200 rounded-full h-2 mt-2">
                <div class="bg-purple-600 h-2 rounded-full" :style="{ width: billing.usage.membersPercent + '%' }"></div>
              </div>
              <div class="text-xs text-gray-500 mt-1">{{ billing.usage.membersPercent }}% of {{ billing.usage.membersLimit }} limit</div>
            </div>
          </div>
        </div>

        <!-- Payment Method -->
        <div class="bg-white rounded-lg shadow-sm p-6">
          <h3 class="text-lg font-medium text-gray-900 mb-4">Payment Method</h3>
          <div class="flex items-center justify-between">
            <div class="flex items-center">
              <div class="w-12 h-8 bg-blue-600 rounded flex items-center justify-center">
                <span class="text-white text-xs font-bold">VISA</span>
              </div>
              <div class="ml-3">
                <div class="text-sm font-medium text-gray-900">•••• •••• •••• {{ billing.paymentMethod.last4 }}</div>
                <div class="text-sm text-gray-500">Expires {{ billing.paymentMethod.expiry }}</div>
              </div>
            </div>
            <button class="px-4 py-2 border border-gray-300 rounded-md text-sm font-medium text-gray-700 hover:bg-gray-50">
              Update
            </button>
          </div>
        </div>
      </div>

      <!-- Security Tab -->
      <div v-if="activeTab === 'security'" class="space-y-6">
        <!-- Two-Factor Authentication -->
        <div class="bg-white rounded-lg shadow-sm p-6">
          <h3 class="text-lg font-medium text-gray-900 mb-4">Two-Factor Authentication</h3>
          <div class="flex items-center justify-between">
            <div>
              <p class="text-sm text-gray-600">Add an extra layer of security to your account</p>
              <span class="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium mt-2"
                    :class="security.twoFactor.enabled ? 'bg-green-100 text-green-800' : 'bg-gray-100 text-gray-800'">
                {{ security.twoFactor.enabled ? 'Enabled' : 'Disabled' }}
              </span>
            </div>
            <button
              @click="toggleTwoFactor"
              :class="[
                'px-4 py-2 rounded-md text-sm font-medium transition-colors duration-200',
                security.twoFactor.enabled
                  ? 'bg-red-600 text-white hover:bg-red-700'
                  : 'bg-blue-600 text-white hover:bg-blue-700'
              ]">
              {{ security.twoFactor.enabled ? 'Disable' : 'Enable' }} 2FA
            </button>
          </div>
        </div>

        <!-- Session Management -->
        <div class="bg-white rounded-lg shadow-sm p-6">
          <h3 class="text-lg font-medium text-gray-900 mb-4">Active Sessions</h3>
          <div class="space-y-4">
            <div v-for="session in security.sessions" :key="session.id" class="flex items-center justify-between p-4 border border-gray-200 rounded-md">
              <div>
                <div class="font-medium text-gray-900">{{ session.device }}</div>
                <div class="text-sm text-gray-500">{{ session.location }} • {{ session.browser }}</div>
                <div class="text-sm text-gray-500">Last active: {{ formatDate(session.lastActive) }}</div>
              </div>
              <div class="flex items-center space-x-2">
                <span v-if="session.current" class="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-green-100 text-green-800">
                  Current
                </span>
                <button
                  v-else
                  @click="terminateSession(session.id)"
                  class="px-3 py-1 border border-gray-300 rounded text-sm text-gray-700 hover:bg-gray-50">
                  Terminate
                </button>
              </div>
            </div>
          </div>
        </div>

        <!-- API Access -->
        <div class="bg-white rounded-lg shadow-sm p-6">
          <h3 class="text-lg font-medium text-gray-900 mb-4">API Access Control</h3>
          <div class="space-y-4">
            <div class="flex items-start">
              <div class="flex items-center h-5">
                <input v-model="security.apiAccess.ipWhitelist" type="checkbox" class="rounded border-gray-300 text-blue-600 focus:ring-blue-500">
              </div>
              <div class="ml-3">
                <label class="text-sm font-medium text-gray-700">IP Whitelist</label>
                <p class="text-sm text-gray-500">Restrict API access to specific IP addresses</p>
              </div>
            </div>
            <div class="flex items-start">
              <div class="flex items-center h-5">
                <input v-model="security.apiAccess.requireSignature" type="checkbox" class="rounded border-gray-300 text-blue-600 focus:ring-blue-500">
              </div>
              <div class="ml-3">
                <label class="text-sm font-medium text-gray-700">Request Signing</label>
                <p class="text-sm text-gray-500">Require cryptographic signatures for API requests</p>
              </div>
            </div>
            <div class="flex items-start">
              <div class="flex items-center h-5">
                <input v-model="security.apiAccess.rateLimit" type="checkbox" class="rounded border-gray-300 text-blue-600 focus:ring-blue-500">
              </div>
              <div class="ml-3">
                <label class="text-sm font-medium text-gray-700">Enhanced Rate Limiting</label>
                <p class="text-sm text-gray-500">Apply stricter rate limits for additional security</p>
              </div>
            </div>
          </div>
        </div>

        <!-- Save Security Settings -->
        <div class="flex justify-end">
          <button
            @click="saveSecuritySettings"
            class="px-6 py-2 bg-blue-600 text-white rounded-md hover:bg-blue-700 transition-colors duration-200">
            Save Security Settings
          </button>
        </div>
      </div>

      <!-- Success/Error Messages -->
      <div v-if="message" class="fixed top-4 right-4 z-50">
        <div :class="[
          'px-4 py-3 rounded-md shadow-lg',
          message.type === 'success' ? 'bg-green-100 text-green-800' : 'bg-red-100 text-red-800'
        ]">
          {{ message.text }}
          <button @click="message = null" class="ml-2 text-sm underline">Close</button>
        </div>
      </div>
    </div>
  </DashboardLayout>
</template>

<script>
import { ref, onMounted } from 'vue'
import DashboardLayout from '../components/DashboardLayout.vue'
import { apiService } from '../services/api'

export default {
  name: 'Account',
  components: {
    DashboardLayout
  },
  setup() {
    const activeTab = ref('organization')
    const message = ref(null)

    // Organization data
    const organization = ref({
      name: 'Acme Corporation',
      industry: 'technology',
      size: '51-200',
      country: 'US',
      description: 'Leading technology company focused on AI and data privacy',
      compliance: {
        gdpr: true,
        hipaa: false,
        ccpa: true,
        soc2: true
      }
    })

    // Team management
    const teamMembers = ref([])
    const newMember = ref({
      email: '',
      role: 'member'
    })

    // Billing information
    const billing = ref({
      plan: {
        name: 'Professional',
        price: 299,
        description: 'Advanced privacy protection for growing teams',
        features: [
          'Unlimited requests',
          'All AI providers',
          'Advanced analytics',
          'Priority support',
          'Custom compliance rules',
          'Team management',
          'API access'
        ]
      },
      usage: {
        requests: 127500,
        requestsLimit: 1000000,
        requestsPercent: 12.8,
        dataGB: 45,
        dataLimit: 100,
        dataPercent: 45,
        membersLimit: 25,
        membersPercent: 32
      },
      paymentMethod: {
        last4: '4242',
        expiry: '12/25',
        type: 'visa'
      }
    })

    // Security settings
    const security = ref({
      twoFactor: {
        enabled: true
      },
      sessions: [
        {
          id: 'sess_001',
          device: 'MacBook Pro',
          location: 'San Francisco, CA',
          browser: 'Chrome 119',
          lastActive: new Date(),
          current: true
        },
        {
          id: 'sess_002',
          device: 'iPhone',
          location: 'San Francisco, CA',
          browser: 'Safari Mobile',
          lastActive: new Date(Date.now() - 3600000),
          current: false
        }
      ],
      apiAccess: {
        ipWhitelist: false,
        requireSignature: true,
        rateLimit: true
      }
    })

    // Load data
    const loadAccountData = async () => {
      try {
        const [orgResponse, teamResponse] = await Promise.all([
          apiService.getOrganization(),
          apiService.getTeamMembers()
        ])

        if (orgResponse.organization) {
          organization.value = { ...organization.value, ...orgResponse.organization }
        }

        teamMembers.value = teamResponse.members || generateSampleTeam()
        billing.value.usage.membersPercent = Math.round((teamMembers.value.filter(m => m.status === 'active').length / billing.value.usage.membersLimit) * 100)
      } catch (error) {
        console.error('Failed to load account data:', error)
        teamMembers.value = generateSampleTeam()
      }
    }

    const generateSampleTeam = () => [
      {
        id: 'member_001',
        name: 'John Doe',
        email: 'john@acme.com',
        role: 'owner',
        status: 'active',
        joined_at: new Date(Date.now() - 86400000 * 30),
        last_active: new Date()
      },
      {
        id: 'member_002',
        name: 'Jane Smith',
        email: 'jane@acme.com',
        role: 'admin',
        status: 'active',
        joined_at: new Date(Date.now() - 86400000 * 15),
        last_active: new Date(Date.now() - 3600000)
      },
      {
        id: 'member_003',
        name: 'Bob Johnson',
        email: 'bob@acme.com',
        role: 'member',
        status: 'active',
        joined_at: new Date(Date.now() - 86400000 * 7),
        last_active: new Date(Date.now() - 86400000)
      },
      {
        id: 'member_004',
        name: 'Alice Wilson',
        email: 'alice@acme.com',
        role: 'member',
        status: 'pending',
        joined_at: new Date(Date.now() - 86400000 * 2),
        last_active: null
      }
    ]

    // Organization operations
    const saveOrganization = async () => {
      try {
        await apiService.updateOrganization(organization.value)
        showMessage('Organization settings saved successfully', 'success')
      } catch (error) {
        showMessage('Failed to save organization settings', 'error')
      }
    }

    // Team operations
    const inviteMember = async () => {
      try {
        const response = await apiService.inviteTeamMember(newMember.value)
        teamMembers.value.push({
          id: response.id || `member_${Date.now()}`,
          name: newMember.value.email.split('@')[0],
          email: newMember.value.email,
          role: newMember.value.role,
          status: 'pending',
          joined_at: new Date(),
          last_active: null
        })

        newMember.value = { email: '', role: 'member' }
        showMessage('Team member invited successfully', 'success')
      } catch (error) {
        showMessage('Failed to invite team member', 'error')
      }
    }

    const updateMemberRole = async (member) => {
      // This would open a modal or inline editor in a real app
      showMessage('Member role update feature coming soon', 'info')
    }

    const removeMember = async (memberId) => {
      try {
        await apiService.removeTeamMember(memberId)
        teamMembers.value = teamMembers.value.filter(m => m.id !== memberId)
        showMessage('Team member removed successfully', 'success')
      } catch (error) {
        showMessage('Failed to remove team member', 'error')
      }
    }

    // Security operations
    const toggleTwoFactor = async () => {
      try {
        const newStatus = !security.value.twoFactor.enabled
        await apiService.updateTwoFactor({ enabled: newStatus })
        security.value.twoFactor.enabled = newStatus
        showMessage(`2FA ${newStatus ? 'enabled' : 'disabled'} successfully`, 'success')
      } catch (error) {
        showMessage('Failed to update 2FA settings', 'error')
      }
    }

    const terminateSession = async (sessionId) => {
      try {
        await apiService.terminateSession(sessionId)
        security.value.sessions = security.value.sessions.filter(s => s.id !== sessionId)
        showMessage('Session terminated successfully', 'success')
      } catch (error) {
        showMessage('Failed to terminate session', 'error')
      }
    }

    const saveSecuritySettings = async () => {
      try {
        await apiService.updateSecuritySettings(security.value.apiAccess)
        showMessage('Security settings saved successfully', 'success')
      } catch (error) {
        showMessage('Failed to save security settings', 'error')
      }
    }

    // Utility functions
    const formatDate = (date) => {
      return new Date(date).toLocaleDateString()
    }

    const getRoleColor = (role) => {
      const colors = {
        'owner': 'bg-purple-100 text-purple-800',
        'admin': 'bg-red-100 text-red-800',
        'member': 'bg-blue-100 text-blue-800',
        'viewer': 'bg-gray-100 text-gray-800'
      }
      return colors[role] || 'bg-gray-100 text-gray-800'
    }

    const getStatusColor = (status) => {
      const colors = {
        'active': 'bg-green-100 text-green-800',
        'pending': 'bg-yellow-100 text-yellow-800',
        'suspended': 'bg-red-100 text-red-800'
      }
      return colors[status] || 'bg-gray-100 text-gray-800'
    }

    const showMessage = (text, type = 'info') => {
      message.value = { text, type }
      setTimeout(() => {
        message.value = null
      }, 5000)
    }

    onMounted(() => {
      loadAccountData()
    })

    return {
      activeTab,
      message,
      organization,
      teamMembers,
      newMember,
      billing,
      security,
      saveOrganization,
      inviteMember,
      updateMemberRole,
      removeMember,
      toggleTwoFactor,
      terminateSession,
      saveSecuritySettings,
      formatDate,
      getRoleColor,
      getStatusColor
    }
  }
}
</script>
