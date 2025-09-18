#!/bin/bash

# Test script for Abdo API
# This script will create test users and orders, then verify they're in the database

BASE_URL="http://localhost:8080/api/v1"

echo "🧪 Testing Abdo API CRUD Operations"
echo "================================="

# Function to check if server is running
check_server() {
    if ! curl -s ${BASE_URL%/api/v1}/health > /dev/null; then
        echo "❌ Server is not running. Please start with 'air' command first."
        exit 1
    fi
    echo "✅ Server is running"
}

# Function to create users
create_users() {
    echo ""
    echo "👥 Creating Users..."
    
    # User 1
    echo "Creating User 1 (John Doe)..."
    curl -X POST $BASE_URL/users \
        -H "Content-Type: application/json" \
        -d '{
            "name": "John Doe",
            "email": "john.doe@example.com",
            "phone": "+1-555-0101",
            "address": "123 Main St, New York, NY 10001"
        }' | jq '.'
    
    # User 2
    echo "Creating User 2 (Jane Smith)..."
    curl -X POST $BASE_URL/users \
        -H "Content-Type: application/json" \
        -d '{
            "name": "Jane Smith",
            "email": "jane.smith@example.com",
            "phone": "+1-555-0102",
            "address": "456 Oak Ave, Los Angeles, CA 90210"
        }' | jq '.'
    
    # User 3
    echo "Creating User 3 (Bob Johnson)..."
    curl -X POST $BASE_URL/users \
        -H "Content-Type: application/json" \
        -d '{
            "name": "Bob Johnson",
            "email": "bob.johnson@example.com",
            "phone": "+1-555-0103",
            "address": "789 Pine Rd, Chicago, IL 60601"
        }' | jq '.'
}

# Function to create orders
create_orders() {
    echo ""
    echo "📦 Creating Orders..."
    
    # Order 1
    echo "Creating Order 1..."
    curl -X POST $BASE_URL/orders \
        -H "Content-Type: application/json" \
        -d '{
            "order_number": "ORD-2025-001",
            "description": "Office supplies and equipment",
            "amount": 299.99,
            "status": "pending",
            "order_date": "2025-09-16T10:00:00Z",
            "user_id": 1
        }' | jq '.'
    
    # Order 2
    echo "Creating Order 2..."
    curl -X POST $BASE_URL/orders \
        -H "Content-Type: application/json" \
        -d '{
            "order_number": "ORD-2025-002",
            "description": "Software licenses",
            "amount": 1599.50,
            "status": "processing",
            "order_date": "2025-09-16T11:30:00Z",
            "user_id": 1
        }' | jq '.'
    
    # Order 3
    echo "Creating Order 3..."
    curl -X POST $BASE_URL/orders \
        -H "Content-Type: application/json" \
        -d '{
            "order_number": "ORD-2025-003",
            "description": "Marketing materials",
            "amount": 750.25,
            "status": "shipped",
            "order_date": "2025-09-15T14:20:00Z",
            "user_id": 2
        }' | jq '.'
    
    # Order 4
    echo "Creating Order 4..."
    curl -X POST $BASE_URL/orders \
        -H "Content-Type: application/json" \
        -d '{
            "order_number": "ORD-2025-004",
            "description": "Hardware components",
            "amount": 2100.00,
            "status": "delivered",
            "order_date": "2025-09-14T16:45:00Z",
            "user_id": 3
        }' | jq '.'
}

# Function to verify data
verify_data() {
    echo ""
    echo "✅ Verifying Data..."
    
    echo "📊 All Users:"
    curl -s $BASE_URL/users | jq '.users | length'
    
    echo "📊 All Orders:"
    curl -s $BASE_URL/orders | jq '.orders | length'
    
    echo "📊 User 1 with Orders:"
    curl -s $BASE_URL/users/1 | jq '.user.orders | length'
}

# Function to test NL→SQL endpoint
test_nl2sql() {
    echo ""
    echo "🧠 Testing NL→SQL endpoint..."
    curl -s -X POST $BASE_URL/nl2sql \
      -H "Content-Type: application/json" \
      -d '{
        "question": "Total order amount by status in the last 30 days; include status and total; descending; top 5"
      }' | jq '.'
}
# Main execution
check_server
create_users
create_orders
verify_data
test_nl2sql

echo ""
echo "🎉 Test completed! Check the output above for results."
echo "💡 You can now test other endpoints manually or view the data in your database."