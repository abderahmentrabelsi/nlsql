# NL to SQL Project

## 🗄️ Database Setup

Currently has a MySQL database with two related tables:

- **Users:** `id`, `name`, `email`, `phone`, `address`  
- **Orders:** `id`, `order_number`, `description`, `amount`, `status`, `order_date`, `user_id`

## 🚀 Quick Start

1. **Setup Environment:**
   ```bash
   cp .env.example .env
   # Update .env with your MySQL database credentials
   ```

2. **Install Dependencies:**
   ```bash
   go mod tidy
   ```

3. **Run Development Server:**
   ```bash
   air
   ```

## 📡 API Endpoints

- `GET /health` - Health check
- `GET /api/v1/users` - List all users  
- `GET /api/v1/orders` - List all orders
- Full CRUD operations for users and orders

## 🔧 Configuration

Configure your environment variables in `.env`:
- `DB_HOST`, `DB_PORT`, `DB_USER`, `DB_PASSWORD`, `DB_NAME`
- `SERVER_PORT` (default: 8080)

## 📋 Requirements

- Go 1.21+
- MySQL 8.0+
- Air (for hot reload development)