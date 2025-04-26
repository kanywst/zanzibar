#!/bin/bash

# Simple client script to demonstrate the Zanzibar API

# Set the API base URL
API_URL="http://localhost:8080"

# Colors for output
GREEN='\033[0;32m'
RED='\033[0;31m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Function to make API calls
call_api() {
  local method=$1
  local endpoint=$2
  local data=$3
  local description=$4

  echo -e "${BLUE}$description${NC}"
  echo "Request: $method $endpoint"
  if [ -n "$data" ]; then
    echo "Data: $data"
  fi

  if [ "$method" == "GET" ]; then
    response=$(curl -s -X GET "$API_URL$endpoint")
  else
    response=$(curl -s -X "$method" -H "Content-Type: application/json" -d "$data" "$API_URL$endpoint")
  fi

  echo "Response: $response"
  echo ""
}

# Check if the server is running
echo -e "${BLUE}Checking if the server is running...${NC}"
health_response=$(curl -s -o /dev/null -w "%{http_code}" "$API_URL/health")
if [ "$health_response" != "200" ]; then
  echo -e "${RED}Server is not running. Please start the server first.${NC}"
  exit 1
fi
echo -e "${GREEN}Server is running.${NC}"
echo ""

# 1. Get the schema
call_api "GET" "/v1/schema" "" "Getting the schema"

# 2. List all relationships
call_api "GET" "/v1/relationships" "" "Listing all relationships"

# 3. Check if Alice can read the report
call_api "POST" "/v1/authorize" '{
  "principal": {"id": "user:alice"},
  "resource": {"id": "document:report"},
  "action": "read"
}' "Checking if Alice can read the report"

# 4. Check if Bob can edit the report
call_api "POST" "/v1/authorize" '{
  "principal": {"id": "user:bob"},
  "resource": {"id": "document:report"},
  "action": "edit"
}' "Checking if Bob can edit the report"

# 5. Check if Charlie can view the report (through direct group membership)
call_api "POST" "/v1/authorize" '{
  "principal": {"id": "user:charlie"},
  "resource": {"id": "document:report"},
  "action": "view"
}' "Checking if Charlie can view the report (through direct group membership)"

# 6. Check if Dave can view the report (through nested group membership)
call_api "POST" "/v1/authorize" '{
  "principal": {"id": "user:dave"},
  "resource": {"id": "document:report"},
  "action": "view"
}' "Checking if Dave can view the report (through nested group membership)"

# 7. Check if Dave can delete the report (should be denied)
call_api "POST" "/v1/authorize" '{
  "principal": {"id": "user:dave"},
  "resource": {"id": "document:report"},
  "action": "delete"
}' "Checking if Dave can delete the report (should be denied)"

# 8. Add a new relationship (Dave as viewer)
call_api "POST" "/v1/relationships" '{
  "resource": {"id": "document:report"},
  "relation": "viewer",
  "subject": {"id": "user:dave"}
}' "Adding Dave as a viewer of the report"

# 9. Check if Dave can now view the report
call_api "POST" "/v1/authorize" '{
  "principal": {"id": "user:dave"},
  "resource": {"id": "document:report"},
  "action": "view"
}' "Checking if Dave can now view the report"

# 10. Get all viewers of the report
call_api "GET" "/v1/resources/document:report/relations/viewer/subjects" "" "Getting all viewers of the report"

# 11. Remove Dave as a viewer
call_api "DELETE" "/v1/relationships" '{
  "resource": {"id": "document:report"},
  "relation": "viewer",
  "subject": {"id": "user:dave"}
}' "Removing Dave as a viewer of the report"

# 12. Check if Dave can still view the report (should be denied)
call_api "POST" "/v1/authorize" '{
  "principal": {"id": "user:dave"},
  "resource": {"id": "document:report"},
  "action": "view"
}' "Checking if Dave can still view the report (should be denied)"

echo -e "${GREEN}All tests completed.${NC}"
