# Test Data for Abdo API

This file contains example data for testing the CRUD operations.

## Users Data

### Create Users (POST /api/v1/users)

User 1:
```json
{
  "name": "John Doe",
  "email": "john.doe@example.com",
  "phone": "+1-555-0101",
  "address": "123 Main St, New York, NY 10001"
}
```

User 2:
```json
{
  "name": "Jane Smith",
  "email": "jane.smith@example.com", 
  "phone": "+1-555-0102",
  "address": "456 Oak Ave, Los Angeles, CA 90210"
}
```

User 3:
```json
{
  "name": "Bob Johnson",
  "email": "bob.johnson@example.com",
  "phone": "+1-555-0103",
  "address": "789 Pine Rd, Chicago, IL 60601"
}
```

## Orders Data

### Create Orders (POST /api/v1/orders)

Order 1 (for User 1):
```json
{
  "order_number": "ORD-2025-001",
  "description": "Office supplies and equipment",
  "amount": 299.99,
  "status": "pending",
  "order_date": "2025-09-16T10:00:00Z",
  "user_id": 1
}
```

Order 2 (for User 1):
```json
{
  "order_number": "ORD-2025-002", 
  "description": "Software licenses",
  "amount": 1599.50,
  "status": "processing",
  "order_date": "2025-09-16T11:30:00Z",
  "user_id": 1
}
```

Order 3 (for User 2):
```json
{
  "order_number": "ORD-2025-003",
  "description": "Marketing materials",
  "amount": 750.25,
  "status": "shipped",
  "order_date": "2025-09-15T14:20:00Z",
  "ship_date": "2025-09-16T09:15:00Z",
  "user_id": 2
}
```

Order 4 (for User 3):
```json
{
  "order_number": "ORD-2025-004",
  "description": "Hardware components",
  "amount": 2100.00,
  "status": "delivered",
  "order_date": "2025-09-14T16:45:00Z",
  "ship_date": "2025-09-15T08:30:00Z",
  "user_id": 3
}
```

## Test API Endpoints

### User CRUD Operations

1. **Create Users** (POST /api/v1/users) - Use the JSON data above
2. **Get All Users** (GET /api/v1/users)
3. **Get User by ID** (GET /api/v1/users/1)
4. **Update User** (PUT /api/v1/users/1):
   ```json
   {
     "name": "John Doe Updated",
     "email": "john.doe.updated@example.com",
     "phone": "+1-555-0101",
     "address": "123 Main St, Updated Address, NY 10001"
   }
   ```
5. **Delete User** (DELETE /api/v1/users/3) - Should fail if user has orders

### Order CRUD Operations

1. **Create Orders** (POST /api/v1/orders) - Use the JSON data above
2. **Get All Orders** (GET /api/v1/orders)
3. **Get Orders by Status** (GET /api/v1/orders?status=shipped)
4. **Get Orders by User** (GET /api/v1/orders?user_id=1)
5. **Get Order by ID** (GET /api/v1/orders/1)
6. **Update Order** (PUT /api/v1/orders/1):
   ```json
   {
     "order_number": "ORD-2025-001-UPDATED",
     "description": "Office supplies and equipment - Updated",
     "amount": 349.99,
     "status": "processing",
     "order_date": "2025-09-16T10:00:00Z",
     "user_id": 1
   }
   ```
7. **Update Order Status** (PATCH /api/v1/orders/1/status):
   ```json
   {
     "status": "shipped"
   }
   ```
8. **Delete Order** (DELETE /api/v1/orders/1)

## Health Check

- **Health Check** (GET /health) - Should return API status