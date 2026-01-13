// AIServe.Farm - Main Application JavaScript
// Handles authentication, GPU rentals, and API interactions

// Configuration
const CONFIG = {
    API_URL: window.location.hostname === 'localhost'
        ? 'http://localhost:8080/api/v1'
        : 'https://api.aiserve.farm/api/v1',
    KEYCLOAK_URL: 'https://auth.afterdarksys.com',
    KEYCLOAK_REALM: 'afterdark',
    KEYCLOAK_CLIENT_ID: 'aiserve-farm'
};

// State management
let authToken = localStorage.getItem('authToken');
let currentUser = null;

// Initialize Keycloak
let keycloak = null;

async function initKeycloak() {
    try {
        // Keycloak initialization (placeholder - will be configured with actual Keycloak settings)
        console.log('Keycloak initialization ready');
        // keycloak = new Keycloak({
        //     url: CONFIG.KEYCLOAK_URL,
        //     realm: CONFIG.KEYCLOAK_REALM,
        //     clientId: CONFIG.KEYCLOAK_CLIENT_ID
        // });
    } catch (error) {
        console.error('Keycloak init failed:', error);
    }
}

// Authentication functions
async function loginWithKeycloak() {
    try {
        // Redirect to After Dark Systems Central Login
        window.location.href = `${CONFIG.KEYCLOAK_URL}/realms/${CONFIG.KEYCLOAK_REALM}/protocol/openid-connect/auth?client_id=${CONFIG.KEYCLOAK_CLIENT_ID}&redirect_uri=${encodeURIComponent(window.location.origin)}&response_type=code&scope=openid`;
    } catch (error) {
        console.error('Keycloak login failed:', error);
        showNotification('Authentication failed. Please try again.', 'error');
    }
}

async function signupWithKeycloak() {
    try {
        // Redirect to After Dark Systems registration
        window.location.href = `${CONFIG.KEYCLOAK_URL}/realms/${CONFIG.KEYCLOAK_REALM}/protocol/openid-connect/registrations?client_id=${CONFIG.KEYCLOAK_CLIENT_ID}&redirect_uri=${encodeURIComponent(window.location.origin)}&response_type=code&scope=openid`;
    } catch (error) {
        console.error('Keycloak signup failed:', error);
        showNotification('Registration failed. Please try again.', 'error');
    }
}

async function login(email, password) {
    try {
        const response = await fetch(`${CONFIG.API_URL}/auth/login`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ email, password })
        });

        if (!response.ok) {
            throw new Error('Login failed');
        }

        const data = await response.json();
        authToken = data.token;
        localStorage.setItem('authToken', authToken);
        currentUser = data.user;

        closeModal('loginModal');
        showNotification('Login successful!', 'success');
        updateUIForAuth();

        return data;
    } catch (error) {
        console.error('Login error:', error);
        showNotification('Login failed. Please check your credentials.', 'error');
        throw error;
    }
}

async function signup(name, email, password) {
    try {
        const response = await fetch(`${CONFIG.API_URL}/auth/register`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ name, email, password })
        });

        if (!response.ok) {
            throw new Error('Signup failed');
        }

        const data = await response.json();
        authToken = data.token;
        localStorage.setItem('authToken', authToken);
        currentUser = data.user;

        closeModal('signupModal');
        showNotification('Account created! Welcome to AIServe.Farm', 'success');
        updateUIForAuth();

        return data;
    } catch (error) {
        console.error('Signup error:', error);
        showNotification('Signup failed. Email may already be in use.', 'error');
        throw error;
    }
}

function logout() {
    authToken = null;
    currentUser = null;
    localStorage.removeItem('authToken');
    updateUIForAuth();
    showNotification('Logged out successfully', 'success');
}

// GPU Management
async function fetchAvailableGPUs() {
    try {
        const response = await fetch(`${CONFIG.API_URL}/gpu/instances`, {
            headers: authToken ? { 'Authorization': `Bearer ${authToken}` } : {}
        });

        if (!response.ok) {
            throw new Error('Failed to fetch GPUs');
        }

        const data = await response.json();
        return data;
    } catch (error) {
        console.error('Error fetching GPUs:', error);
        return [];
    }
}

async function rentGPU(gpuType, duration) {
    if (!authToken) {
        showNotification('Please login to rent GPUs', 'error');
        openModal('loginModal');
        return;
    }

    try {
        showNotification('Reserving GPU...', 'info');

        const response = await fetch(`${CONFIG.API_URL}/gpu/instances/reserve`, {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
                'Authorization': `Bearer ${authToken}`
            },
            body: JSON.stringify({
                gpu_type: gpuType,
                duration: duration || 3600, // Default 1 hour
                provider: 'vast.ai'
            })
        });

        if (!response.ok) {
            throw new Error('GPU rental failed');
        }

        const data = await response.json();
        showNotification(`GPU ${gpuType} reserved successfully! Instance ID: ${data.instance_id}`, 'success');

        // Redirect to dashboard or show connection info
        showGPUConnectionInfo(data);

        return data;
    } catch (error) {
        console.error('Rent GPU error:', error);
        showNotification('Failed to rent GPU. Please try again.', 'error');
        throw error;
    }
}

function showGPUConnectionInfo(instanceData) {
    const modal = document.createElement('div');
    modal.className = 'modal active';
    modal.innerHTML = `
        <div class="modal-content">
            <span class="modal-close" onclick="this.parentElement.parentElement.remove()">&times;</span>
            <h2>🎉 GPU Reserved!</h2>
            <p class="modal-subtitle">Your instance is ready</p>
            <div style="background: var(--bg-dark); padding: 1.5rem; border-radius: 8px; margin: 1.5rem 0;">
                <p style="margin-bottom: 0.5rem;"><strong>Instance ID:</strong> ${instanceData.instance_id}</p>
                <p style="margin-bottom: 0.5rem;"><strong>SSH:</strong> <code>ssh root@${instanceData.ip_address || 'pending'}</code></p>
                <p style="margin-bottom: 0.5rem;"><strong>GPU:</strong> ${instanceData.gpu_type}</p>
                <p><strong>Status:</strong> <span class="badge badge-available">Active</span></p>
            </div>
            <button class="btn btn-primary btn-full" onclick="window.location.href='/dashboard'">
                Go to Dashboard
            </button>
        </div>
    `;
    document.body.appendChild(modal);
}

// UI Functions
function openModal(modalId) {
    const modal = document.getElementById(modalId);
    if (modal) {
        modal.classList.add('active');
    }
}

function closeModal(modalId) {
    const modal = document.getElementById(modalId);
    if (modal) {
        modal.classList.remove('active');
    }
}

function showNotification(message, type = 'info') {
    const notification = document.createElement('div');
    notification.style.cssText = `
        position: fixed;
        top: 80px;
        right: 20px;
        background: ${type === 'success' ? 'var(--success)' : type === 'error' ? 'var(--error)' : 'var(--primary)'};
        color: white;
        padding: 1rem 1.5rem;
        border-radius: 8px;
        box-shadow: 0 10px 25px rgba(0,0,0,0.3);
        z-index: 9999;
        animation: slideIn 0.3s ease;
    `;
    notification.textContent = message;
    document.body.appendChild(notification);

    setTimeout(() => {
        notification.style.animation = 'slideOut 0.3s ease';
        setTimeout(() => notification.remove(), 300);
    }, 3000);
}

function updateUIForAuth() {
    const loginBtn = document.getElementById('loginBtn');
    const signupBtn = document.getElementById('signupBtn');

    if (authToken && currentUser) {
        loginBtn.textContent = currentUser.name || currentUser.email;
        loginBtn.onclick = () => window.location.href = '/dashboard';
        signupBtn.textContent = 'Dashboard';
        signupBtn.onclick = () => window.location.href = '/dashboard';
    } else {
        loginBtn.textContent = 'Login';
        loginBtn.onclick = () => openModal('loginModal');
        signupBtn.textContent = 'Sign Up';
        signupBtn.onclick = () => openModal('signupModal');
    }
}

// Event Listeners
document.addEventListener('DOMContentLoaded', () => {
    // Initialize Keycloak
    initKeycloak();

    // Navigation buttons
    document.getElementById('loginBtn')?.addEventListener('click', () => openModal('loginModal'));
    document.getElementById('signupBtn')?.addEventListener('click', () => openModal('signupModal'));
    document.getElementById('getStartedBtn')?.addEventListener('click', () => openModal('signupModal'));

    // Keycloak buttons
    document.getElementById('keycloakLoginBtn')?.addEventListener('click', loginWithKeycloak);
    document.getElementById('keycloakSignupBtn')?.addEventListener('click', signupWithKeycloak);

    // Modal close buttons
    document.querySelectorAll('.modal-close').forEach(btn => {
        btn.addEventListener('click', (e) => {
            e.target.closest('.modal').classList.remove('active');
        });
    });

    // Modal switch links
    document.getElementById('switchToSignup')?.addEventListener('click', (e) => {
        e.preventDefault();
        closeModal('loginModal');
        openModal('signupModal');
    });

    document.getElementById('switchToLogin')?.addEventListener('click', (e) => {
        e.preventDefault();
        closeModal('signupModal');
        openModal('loginModal');
    });

    // Login form
    document.getElementById('loginForm')?.addEventListener('submit', async (e) => {
        e.preventDefault();
        const email = e.target.querySelector('input[type="email"]').value;
        const password = e.target.querySelector('input[type="password"]').value;
        await login(email, password);
    });

    // Signup form
    document.getElementById('signupForm')?.addEventListener('submit', async (e) => {
        e.preventDefault();
        const name = e.target.querySelector('input[type="text"]').value;
        const email = e.target.querySelector('input[type="email"]').value;
        const password = e.target.querySelector('input[type="password"]').value;
        await signup(name, email, password);
    });

    // Rent buttons
    document.querySelectorAll('.rent-btn').forEach(btn => {
        btn.addEventListener('click', async (e) => {
            const gpuType = e.target.dataset.gpu;
            await rentGPU(gpuType);
        });
    });

    // View all GPUs button
    document.getElementById('viewAllGPUs')?.addEventListener('click', () => {
        window.location.href = '#models';
        // In future: load more GPUs dynamically
    });

    // Close modals when clicking outside
    window.addEventListener('click', (e) => {
        if (e.target.classList.contains('modal')) {
            e.target.classList.remove('active');
        }
    });

    // Check for auth token on load
    if (authToken) {
        // Verify token is still valid
        fetch(`${CONFIG.API_URL}/user/export`, {
            headers: { 'Authorization': `Bearer ${authToken}` }
        })
        .then(response => {
            if (response.ok) {
                return response.json();
            } else {
                throw new Error('Token invalid');
            }
        })
        .then(data => {
            currentUser = data.user;
            updateUIForAuth();
        })
        .catch(() => {
            // Token invalid, clear it
            logout();
        });
    } else {
        updateUIForAuth();
    }

    // Load GPUs dynamically (future enhancement)
    // fetchAvailableGPUs().then(gpus => {
    //     // Populate GPU grid with real data
    // });

    // Handle Keycloak callback
    const urlParams = new URLSearchParams(window.location.search);
    if (urlParams.has('code')) {
        // Exchange code for token
        showNotification('Authenticating with After Dark Systems...', 'info');
        // Handle OAuth callback
        // This would exchange the code for a token with your backend
    }
});

// Smooth scrolling for anchor links
document.querySelectorAll('a[href^="#"]').forEach(anchor => {
    anchor.addEventListener('click', function (e) {
        e.preventDefault();
        const target = document.querySelector(this.getAttribute('href'));
        if (target) {
            target.scrollIntoView({ behavior: 'smooth', block: 'start' });
        }
    });
});

// Add CSS animations
const style = document.createElement('style');
style.textContent = `
    @keyframes slideIn {
        from {
            transform: translateX(100%);
            opacity: 0;
        }
        to {
            transform: translateX(0);
            opacity: 1;
        }
    }

    @keyframes slideOut {
        from {
            transform: translateX(0);
            opacity: 1;
        }
        to {
            transform: translateX(100%);
            opacity: 0;
        }
    }
`;
document.head.appendChild(style);
