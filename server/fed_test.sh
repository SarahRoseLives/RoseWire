curl -X POST http://localhost:8080/api/s2s/inbox \
-H "Content-Type: application/json" \
-H "Authorization: Bearer a-very-secret-and-long-random-string-that-you-create" \
-d '{
  "type": "Create",
  "actor": "@federated_friend@another.instance",
  "object": {
    "content": "This is a federated message!"
  }
}'