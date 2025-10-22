// Profile management JavaScript
document.addEventListener('DOMContentLoaded', function() {
    const profileForm = document.getElementById('profileForm');
    const profilePictureInput = document.getElementById('profilePictureInput');
    const profileImage = document.getElementById('profileImage');

    // Handle profile picture upload
    if (profilePictureInput) {
        profilePictureInput.addEventListener('change', function(e) {
            const file = e.target.files[0];
            if (file) {
                // Validate file type
                if (!file.type.startsWith('image/')) {
                    showAlert('Please select a valid image file.', 'error');
                    return;
                }

                // Validate file size (5MB max)
                if (file.size > 5 * 1024 * 1024) {
                    showAlert('Image file size must be less than 5MB.', 'error');
                    return;
                }

                // Preview the image
                const reader = new FileReader();
                reader.onload = function(e) {
                    profileImage.src = e.target.result;
                };
                reader.readAsDataURL(file);

                // Upload the image (you can implement this based on your file upload strategy)
                uploadProfilePicture(file);
            }
        });
    }

    // Handle profile form submission
    if (profileForm) {
        profileForm.addEventListener('submit', function(e) {
            e.preventDefault();
            updateProfile();
        });
    }

    // Auto-save functionality (optional)
    const formInputs = profileForm.querySelectorAll('input, select, textarea');
    formInputs.forEach(input => {
        input.addEventListener('change', function() {
            // Mark form as dirty
            profileForm.classList.add('form-dirty');
        });
    });

    // Warn user about unsaved changes
    window.addEventListener('beforeunload', function(e) {
        if (profileForm.classList.contains('form-dirty')) {
            e.preventDefault();
            e.returnValue = 'You have unsaved changes. Are you sure you want to leave?';
            return e.returnValue;
        }
    });
});

// Update user profile
async function updateProfile() {
    const form = document.getElementById('profileForm');
    const formData = new FormData(form);
    
    // Convert FormData to JSON
    const profileData = {};
    for (let [key, value] of formData.entries()) {
        if (value.trim() !== '') {
            profileData[key] = value;
        }
    }

    try {
        showLoading(true);
        
        const token = getAuthToken();
        if (!token) {
            showAlert('Authentication required. Please login again.', 'error');
            window.location.href = '/login';
            return;
        }

        const response = await fetch('/api/v1/profile', {
            method: 'PUT',
            headers: {
                'Content-Type': 'application/json',
                'Authorization': `Bearer ${token}`
            },
            body: JSON.stringify(profileData)
        });

        const result = await response.json();

        if (response.ok) {
            showAlert(result.message || 'Profile updated successfully!', 'success');
            form.classList.remove('form-dirty');
            
            // Update the display name if changed
            updateDisplayName(profileData);
        } else {
            showAlert(result.message || 'Failed to update profile', 'error');
        }
    } catch (error) {
        console.error('Error updating profile:', error);
        showAlert('Network error. Please try again.', 'error');
    } finally {
        showLoading(false);
    }
}

// Upload profile picture
async function uploadProfilePicture(file) {
    try {
        showLoading(true);
        
        const token = getAuthToken();
        if (!token) {
            showAlert('Authentication required. Please login again.', 'error');
            return;
        }

        // For now, we'll use a placeholder URL
        // In a real implementation, you would upload to a file storage service
        const formData = new FormData();
        formData.append('profile_picture', file);

        // Simulate upload delay
        await new Promise(resolve => setTimeout(resolve, 1000));
        
        // For demo purposes, we'll use a placeholder service
        const placeholderUrl = `https://via.placeholder.com/150x150/667eea/ffffff?text=${encodeURIComponent(getUserInitials())}`;
        
        // Update profile with new picture URL
        const response = await fetch('/api/v1/profile', {
            method: 'PUT',
            headers: {
                'Content-Type': 'application/json',
                'Authorization': `Bearer ${token}`
            },
            body: JSON.stringify({
                profile_picture_url: placeholderUrl
            })
        });

        if (response.ok) {
            showAlert('Profile picture updated successfully!', 'success');
        } else {
            showAlert('Failed to update profile picture', 'error');
        }
    } catch (error) {
        console.error('Error uploading profile picture:', error);
        showAlert('Failed to upload profile picture', 'error');
    } finally {
        showLoading(false);
    }
}

// Get user initials for placeholder
function getUserInitials() {
    const firstName = document.getElementById('firstName')?.value || '';
    const lastName = document.getElementById('lastName')?.value || '';
    
    if (firstName && lastName) {
        return (firstName.charAt(0) + lastName.charAt(0)).toUpperCase();
    }
    
    const username = document.querySelector('.user-basic-info h2')?.textContent || 'U';
    return username.charAt(0).toUpperCase();
}

// Update display name in the UI
function updateDisplayName(profileData) {
    const displayNameElement = document.querySelector('.user-basic-info h2');
    if (displayNameElement && (profileData.first_name || profileData.last_name)) {
        const firstName = profileData.first_name || '';
        const lastName = profileData.last_name || '';
        if (firstName || lastName) {
            displayNameElement.textContent = `${firstName} ${lastName}`.trim();
        }
    }
}

// Get authentication token from localStorage or cookie
function getAuthToken() {
    // First try localStorage
    let token = localStorage.getItem('auth_token');
    
    // If not found, try cookie
    if (!token) {
        const cookies = document.cookie.split(';');
        for (let cookie of cookies) {
            const [name, value] = cookie.trim().split('=');
            if (name === 'auth_token') {
                token = value;
                break;
            }
        }
    }
    
    return token;
}

// Show loading state
function showLoading(show) {
    const form = document.getElementById('profileForm');
    const submitBtn = form.querySelector('button[type="submit"]');
    
    if (show) {
        form.classList.add('form-loading');
        submitBtn.disabled = true;
        submitBtn.innerHTML = '<div class="loading-spinner"></div> Saving...';
    } else {
        form.classList.remove('form-loading');
        submitBtn.disabled = false;
        submitBtn.innerHTML = 'Save Changes';
    }
}

// Show alert messages
function showAlert(message, type = 'info') {
    // Remove existing alerts
    const existingAlerts = document.querySelectorAll('.alert');
    existingAlerts.forEach(alert => alert.remove());
    
    // Create new alert
    const alert = document.createElement('div');
    alert.className = `alert alert-${type}`;
    alert.textContent = message;
    
    // Insert after profile header
    const profileHeader = document.querySelector('.profile-header');
    profileHeader.insertAdjacentElement('afterend', alert);
    
    // Auto-remove after 5 seconds
    setTimeout(() => {
        if (alert.parentNode) {
            alert.remove();
        }
    }, 5000);
    
    // Scroll to alert
    alert.scrollIntoView({ behavior: 'smooth', block: 'nearest' });
}

// Validate form fields
function validateForm() {
    const form = document.getElementById('profileForm');
    const emailRegex = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;
    const urlRegex = /^https?:\/\/.+/;
    
    // Validate email format in URLs
    const urlFields = ['website', 'linkedinUrl', 'twitterUrl', 'githubUrl'];
    urlFields.forEach(fieldId => {
        const field = document.getElementById(fieldId);
        if (field && field.value && !urlRegex.test(field.value)) {
            showAlert(`Please enter a valid URL for ${field.previousElementSibling.textContent}`, 'error');
            field.focus();
            return false;
        }
    });
    
    // Validate phone number format (basic)
    const phoneField = document.getElementById('phone');
    if (phoneField && phoneField.value) {
        const phoneRegex = /^[\+]?[1-9][\d]{0,15}$/;
        if (!phoneRegex.test(phoneField.value.replace(/[\s\-\(\)]/g, ''))) {
            showAlert('Please enter a valid phone number', 'error');
            phoneField.focus();
            return false;
        }
    }
    
    return true;
}

// Format phone number as user types
document.getElementById('phone')?.addEventListener('input', function(e) {
    let value = e.target.value.replace(/\D/g, '');
    if (value.length >= 6) {
        value = value.replace(/(\d{3})(\d{3})(\d{4})/, '($1) $2-$3');
    } else if (value.length >= 3) {
        value = value.replace(/(\d{3})(\d{3})/, '($1) $2');
    }
    e.target.value = value;
});

// Auto-capitalize names
['firstName', 'lastName', 'city', 'state', 'country'].forEach(fieldId => {
    const field = document.getElementById(fieldId);
    if (field) {
        field.addEventListener('input', function(e) {
            const words = e.target.value.split(' ');
            const capitalizedWords = words.map(word => 
                word.charAt(0).toUpperCase() + word.slice(1).toLowerCase()
            );
            e.target.value = capitalizedWords.join(' ');
        });
    }
});

// Character count for bio
const bioField = document.getElementById('bio');
if (bioField) {
    const maxLength = 500;
    const charCount = document.createElement('div');
    charCount.className = 'char-count';
    charCount.style.cssText = 'text-align: right; font-size: 0.8rem; color: #666; margin-top: 0.25rem;';
    bioField.parentNode.appendChild(charCount);
    
    function updateCharCount() {
        const remaining = maxLength - bioField.value.length;
        charCount.textContent = `${remaining} characters remaining`;
        charCount.style.color = remaining < 50 ? '#dc3545' : '#666';
    }
    
    bioField.addEventListener('input', updateCharCount);
    bioField.setAttribute('maxlength', maxLength);
    updateCharCount();
}