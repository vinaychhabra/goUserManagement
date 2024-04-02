#User Management System with Invitation Code

##Overview

This Go package provides functionalities for user management, including user registration, login, and admin management. It includes HTTP handlers to handle user registration, login, admin registration, and invitation code generation.

##Features

User registration: Allows users to register with a unique username and password.
User login: Enables users to authenticate themselves with their registered credentials.
Admin registration: Provides functionality for registering admin users with special privileges.
Invitation code generation: Generates unique invitation codes for user registration.

##Installation

To use this package, you need to have Go installed on your system. You can install the package using the go get command:

```bash
go get github.com/your-username/your-package-name
Replace github.com/your-username/your-package-name with the actual path to your package.
```
##Usage

Once installed, you can import the package in your Go code and use its functionalities. Below is an example of how to use the package to set up a user management server:

```bash
package main

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"path/filepath"

	_ "github.com/lib/pq"
)

func main() {
	// Set up database connection
	db := SetupDatabase()
	defer db.Close()

	// Set up HTTP handlers
	http.HandleFunc("/register", RegisterHandler(db))
	http.HandleFunc("/login", LoginHandler(db))
	http.HandleFunc("/generate-invitation", GenerateInvitationHandler(db))
	http.HandleFunc("/register-admin", RegisterAdminHandler(db))
	http.HandleFunc("/invite", invitePageHandler)
	http.HandleFunc("/", StaticFileHandler)

	// Start server
	log.Println("Server started on :8081")
	log.Fatal(http.ListenAndServe(":8081", nil))
}

```
Make sure to replace SetupDatabase, RegisterHandler, LoginHandler, GenerateInvitationHandler, RegisterAdminHandler, invitePageHandler, and StaticFileHandler with your actual implementations.

##Endpoints

- /register: Endpoint for user registration. Expects a POST request with JSON containing username, password, and invitation_code fields.
- /login: Endpoint for user login. Expects a POST request with JSON containing username and password fields.
- /generate-invitation: Endpoint for generating an invitation code. Expects a POST request with JSON containing admin credentials (username and password fields).
- /register-admin: Endpoint for registering an admin user. Expects a POST request with JSON containing username and password fields.
- /invite: Endpoint for serving the invite page .
- /: Endpoint for serving static files (frontend). This serves the main index.html file.
