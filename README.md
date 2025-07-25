# Authentication Microservices

This project consists of two Go microservices that provide user authentication functionality:

1. **Auth Frontend Service** - A web interface for login, signup, and password management
2. **Auth Backend Service** - A REST API that handles authentication logic

## Features

- **User Login** - Secure user authentication with JWT tokens
- **User Signup** - New user registration with email validation
- **Password Change** - Secure password update functionality
- **Modern UI** - Responsive web interface with modern design
- **Microservice Architecture** - Separate frontend and backend services
- **JWT Authentication** - Secure token-based authentication
- **Password Hashing** - Bcrypt encryption for password security
- **Docker Support** - Containerized deployment with Docker Compose

## Architecture

```
┌─────────────────┐    HTTP/JSON    ┌─────────────────┐
│                 │    Requests     │                 │
│  Auth Frontend  │ ──────────────► │  Auth Backend   │
│   (Port 8080)   │                 │   (Port 8081)   │
│                 │                 │                 │
└─────────────────┘                 └─────────────────┘
```

## API Endpoints

### Auth Backend Service (Port 8081)

- `POST /login` - User authentication
- `POST /signup` - User registration
- `POST /change-password` - Password change
- `GET /health` - Health check

### Auth Frontend Service (Port 8080)

- `GET /` - Login page (redirects to /login)
- `GET /login` - Login page
- `POST /login` - Handle login form
- `GET /signup` - Signup page
- `POST /signup` - Handle signup form
- `GET /change-password` - Change password page
- `POST /change-password` - Handle password change
- `GET /dashboard` - User dashboard (requires authentication)
- `POST /logout` - User logout

## Quick Start

### Using Docker Compose (Recommended)

1. Clone the repository:
   ```bash
   git clone <repository-url>
   cd auth-microservices
   ```

2. Start the services:
   ```bash
   docker-compose up --build
   ```

3. Access the application:
   - Frontend: http://localhost:8080
   - Backend: http://localhost:8081

### Manual Setup

#### Prerequisites

- Go 1.21 or higher
- Git

#### Running the Auth Backend Service

1. Navigate to the auth service directory:
   ```bash
   cd auth-service
   ```

2. Install dependencies:
   ```bash
   go mod tidy
   ```

3. Run the service:
   ```bash
   go run main.go
   ```

The auth service will start on port 8081.

#### Running the Auth Frontend Service

1. Navigate to the project root:
   ```bash
   cd ..
   ```

2. Install dependencies:
   ```bash
   go mod tidy
   ```

3. Set the auth service URL (optional):
   ```bash
   export AUTH_SERVICE_URL=http://localhost:8081
   ```

4. Run the service:
   ```bash
   go run main.go
   ```

The frontend service will start on port 8080.

## Usage

1. **Access the Application**: Open your browser and go to http://localhost:8080

2. **Create an Account**: 
   - Click "Don't have an account? Sign up"
   - Fill in the signup form with username, email, and password
   - Click "Create Account"

3. **Login**:
   - Enter your username and password
   - Click "Sign In"
   - You'll be redirected to the dashboard

4. **Change Password**:
   - From the dashboard, click "Change Password"
   - Or access directly from the login page
   - Enter your username, current password, and new password

5. **Logout**:
   - From the dashboard, click "Logout"

## Configuration

### Environment Variables

#### Auth Frontend Service
- `PORT` - Port to run the frontend service (default: 8080)
- `AUTH_SERVICE_URL` - URL of the auth backend service (default: http://localhost:8081)

#### Auth Backend Service
- `AUTH_PORT` - Port to run the auth service (default: 8081)

### Security Configuration

- JWT tokens expire after 24 hours
- Passwords are hashed using bcrypt with default cost
- In production, set a secure JWT secret via environment variable

## Development

### Project Structure

```
.
├── main.go                 # Frontend service main file
├── go.mod                  # Frontend service dependencies
├── templates/              # HTML templates
│   ├── login.html
│   ├── signup.html
│   ├── change-password.html
│   └── dashboard.html
├── static/                 # Static assets
│   └── css/
│       └── style.css
├── auth-service/           # Backend service
│   ├── main.go
│   └── go.mod
├── Dockerfile.frontend     # Frontend Dockerfile
├── Dockerfile.auth         # Backend Dockerfile
├── docker-compose.yml      # Docker Compose configuration
└── README.md
```

### Adding Features

1. **Database Integration**: Replace the in-memory user store with a database (PostgreSQL, MySQL, etc.)
2. **Email Verification**: Add email verification for new signups
3. **Password Reset**: Implement forgot password functionality
4. **Rate Limiting**: Add rate limiting to prevent brute force attacks
5. **OAuth Integration**: Add social login (Google, GitHub, etc.)
6. **User Profiles**: Extend user model with additional profile information

## Testing

### Manual Testing

1. Start both services
2. Test user registration
3. Test user login
4. Test password change
5. Test logout functionality

### API Testing with curl

```bash
# Test signup
curl -X POST http://localhost:8081/signup \
  -H "Content-Type: application/json" \
  -d '{"username":"testuser","password":"testpass","email":"test@example.com"}'

# Test login
curl -X POST http://localhost:8081/login \
  -H "Content-Type: application/json" \
  -d '{"username":"testuser","password":"testpass"}'

# Test password change
curl -X POST http://localhost:8081/change-password \
  -H "Content-Type: application/json" \
  -d '{"username":"testuser","old_password":"testpass","new_password":"newpass"}'
```

## Production Deployment

1. **Environment Variables**: Set secure JWT secrets and database connections
2. **HTTPS**: Use HTTPS in production with proper SSL certificates
3. **Database**: Use a production database instead of in-memory storage
4. **Monitoring**: Add logging and monitoring (Prometheus, Grafana)
5. **Load Balancing**: Use a load balancer for high availability
6. **Security**: Implement additional security measures (rate limiting, CORS configuration)

## Troubleshooting

### Common Issues

1. **Connection Refused**: Make sure both services are running and accessible
2. **CORS Errors**: Check the CORS configuration in the auth service
3. **Template Not Found**: Ensure the templates directory is in the correct location
4. **Port Already in Use**: Change the port numbers if they're already in use

### Logs

Both services provide detailed logging. Check the console output for error messages and debugging information.

## License

This project is open source and available under the MIT License.