#!/bin/bash

# Configuration variables
CONTAINER_NAME="UserManagementDB" 
POSTGRES_USER="root"
POSTGRES_PASSWORD="pass123456"
POSTGRES_DB="users"
#these credentials you can change
TABLE_USERS_SQL="CREATE TABLE IF NOT EXISTS users (id SERIAL PRIMARY KEY, username VARCHAR(255) UNIQUE NOT NULL, password_hash CHAR(60) NOT NULL);"
TABLE_INVITATIONS_SQL="CREATE TABLE IF NOT EXISTS invitations (
    id SERIAL PRIMARY KEY,
    code VARCHAR(255) UNIQUE NOT NULL,
    used BOOLEAN NOT NULL DEFAULT false,
    issued_at TIMESTAMP NOT NULL
);"
TABLE_ADMINS_SQL="CREATE TABLE IF NOT EXISTS admins (id SERIAL PRIMARY KEY, username VARCHAR(255) UNIQUE NOT NULL, password_hash CHAR(60) NOT NULL);"

# Check if container already exists
if [ $(docker ps -a -f name=^/${CONTAINER_NAME}$ --format '{{.Names}}') == $CONTAINER_NAME ]; then
    echo "Container $CONTAINER_NAME already exists. Stopping and removing it."
    docker stop $CONTAINER_NAME
    docker rm $CONTAINER_NAME
fi

# Pull the PostgreSQL Docker image
echo "Pulling the PostgreSQL Docker image..."
docker pull postgres

# Run the PostgreSQL container
echo "Running the PostgreSQL Docker container..."
docker run --name $CONTAINER_NAME -e POSTGRES_USER=$POSTGRES_USER -e POSTGRES_PASSWORD=$POSTGRES_PASSWORD -e POSTGRES_DB=$POSTGRES_DB -p 5432:5432 -d postgres

# Wait for PostgreSQL to start
echo "Waiting for PostgreSQL to start..."
sleep 10

# Execute the SQL to create the users, invitations, and admins tables
echo "Creating the users, invitations, and admins tables in the $POSTGRES_DB database..."
docker exec -it $CONTAINER_NAME psql -U $POSTGRES_USER -d $POSTGRES_DB -c "$TABLE_USERS_SQL"
docker exec -it $CONTAINER_NAME psql -U $POSTGRES_USER -d $POSTGRES_DB -c "$TABLE_INVITATIONS_SQL"
docker exec -it $CONTAINER_NAME psql -U $POSTGRES_USER -d $POSTGRES_DB -c "$TABLE_ADMINS_SQL"

echo "PostgreSQL setup completed successfully."
