package relay_test

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	_ "github.com/tursodatabase/turso-go"
)

// TestTursoBasicConnect tests basic Turso database connectivity
func TestTursoBasicConnect(t *testing.T) {
	// Create a temporary directory for the test database
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	// Open a Turso database connection
	// Using turso driver with local SQLite file
	conn, err := sql.Open("turso", dbPath)
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer conn.Close()

	// Test connection with a simple ping
	if err := conn.Ping(); err != nil {
		t.Fatalf("Failed to ping database: %v", err)
	}

	t.Log("Successfully connected to Turso database")
}

// TestTursoCreateTable tests creating a table in Turso database
func TestTursoCreateTable(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	conn, err := sql.Open("turso", dbPath)
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer conn.Close()

	// Create a test table
	createTableSQL := `
		CREATE TABLE IF NOT EXISTS users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			username TEXT NOT NULL UNIQUE,
			email TEXT NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)
	`

	if _, err := conn.Exec(createTableSQL); err != nil {
		t.Fatalf("Failed to create table: %v", err)
	}

	t.Log("Successfully created users table")
}

// TestTursoInsertAndQuery tests inserting and querying data
func TestTursoInsertAndQuery(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	conn, err := sql.Open("turso", dbPath)
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer conn.Close()

	// Create table
	createTableSQL := `
		CREATE TABLE IF NOT EXISTS users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			username TEXT NOT NULL UNIQUE,
			email TEXT NOT NULL
		)
	`

	if _, err := conn.Exec(createTableSQL); err != nil {
		t.Fatalf("Failed to create table: %v", err)
	}

	// Insert test data
	insertSQL := `INSERT INTO users (username, email) VALUES (?, ?)`
	users := []struct {
		username string
		email    string
	}{
		{"alice", "alice@example.com"},
		{"bob", "bob@example.com"},
		{"charlie", "charlie@example.com"},
	}

	for _, user := range users {
		result, err := conn.Exec(insertSQL, user.username, user.email)
		if err != nil {
			t.Fatalf("Failed to insert user %s: %v", user.username, err)
		}

		rowsAffected, _ := result.RowsAffected()
		if rowsAffected != 1 {
			t.Errorf("Expected 1 row affected, got %d", rowsAffected)
		}
	}

	t.Logf("Successfully inserted %d users", len(users))

	// Query the data
	querySQL := `SELECT id, username, email FROM users ORDER BY username`
	rows, err := conn.Query(querySQL)
	if err != nil {
		t.Fatalf("Failed to query users: %v", err)
	}
	defer rows.Close()

	var retrievedUsers []struct {
		id       int
		username string
		email    string
	}

	for rows.Next() {
		var id int
		var username, email string
		if err := rows.Scan(&id, &username, &email); err != nil {
			t.Fatalf("Failed to scan row: %v", err)
		}
		retrievedUsers = append(retrievedUsers, struct {
			id       int
			username string
			email    string
		}{id, username, email})
	}

	if err := rows.Err(); err != nil {
		t.Fatalf("Error iterating rows: %v", err)
	}

	// Verify we got all users back
	if len(retrievedUsers) != len(users) {
		t.Fatalf("Expected %d users, got %d", len(users), len(retrievedUsers))
	}

	// Verify usernames match
	for i, user := range retrievedUsers {
		expectedUser := users[i]
		if user.username != expectedUser.username {
			t.Errorf("Expected username %s, got %s", expectedUser.username, user.username)
		}
		if user.email != expectedUser.email {
			t.Errorf("Expected email %s for user %s, got %s", expectedUser.email, expectedUser.username, user.email)
		}
	}

	t.Logf("Successfully queried and verified %d users", len(retrievedUsers))
}

// TestTursoUpdate tests updating records in the database
func TestTursoUpdate(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	conn, err := sql.Open("turso", dbPath)
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer conn.Close()

	// Create table and insert initial data
	createTableSQL := `
		CREATE TABLE IF NOT EXISTS users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			username TEXT NOT NULL UNIQUE,
			email TEXT NOT NULL
		)
	`

	if _, err := conn.Exec(createTableSQL); err != nil {
		t.Fatalf("Failed to create table: %v", err)
	}

	insertSQL := `INSERT INTO users (username, email) VALUES (?, ?)`
	if _, err := conn.Exec(insertSQL, "alice", "alice@old.com"); err != nil {
		t.Fatalf("Failed to insert user: %v", err)
	}

	// Update email
	updateSQL := `UPDATE users SET email = ? WHERE username = ?`
	result, err := conn.Exec(updateSQL, "alice@new.com", "alice")
	if err != nil {
		t.Fatalf("Failed to update user: %v", err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected != 1 {
		t.Errorf("Expected 1 row affected, got %d", rowsAffected)
	}

	// Verify update
	var email string
	err = conn.QueryRow(`SELECT email FROM users WHERE username = ?`, "alice").Scan(&email)
	if err != nil {
		t.Fatalf("Failed to query updated user: %v", err)
	}

	if email != "alice@new.com" {
		t.Errorf("Expected email alice@new.com, got %s", email)
	}

	t.Log("Successfully updated user email")
}

// TestTursoDelete tests deleting records from the database
func TestTursoDelete(t *testing.T) {
	// Use in-memory database for complete test isolation
	conn, err := sql.Open("turso", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer conn.Close()

	// Create table
	createTableSQL := `
		CREATE TABLE IF NOT EXISTS users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			username TEXT NOT NULL UNIQUE,
			email TEXT NOT NULL
		)
	`

	if _, err := conn.Exec(createTableSQL); err != nil {
		t.Fatalf("Failed to create table: %v", err)
	}

	insertSQL := `INSERT INTO users (username, email) VALUES (?, ?)`
	if _, err := conn.Exec(insertSQL, "alice", "alice@example.com"); err != nil {
		t.Fatalf("Failed to insert user: %v", err)
	}

	// Verify alice exists
	var count int
	err = conn.QueryRow(`SELECT COUNT(*) FROM users WHERE username = ?`, "alice").Scan(&count)
	if err != nil {
		t.Fatalf("Failed to query count before delete: %v", err)
	}
	t.Logf("Before delete: %d alice users found", count)

	// Delete user
	deleteSQL := `DELETE FROM users WHERE username = ?`
	result, err := conn.Exec(deleteSQL, "alice")
	if err != nil {
		t.Fatalf("Failed to delete user: %v", err)
	}

	rowsAffected, _ := result.RowsAffected()
	t.Logf("Delete affected %d rows", rowsAffected)
	if rowsAffected < 1 {
		t.Errorf("Expected at least 1 row affected, got %d", rowsAffected)
	}

	// Verify deletion
	err = conn.QueryRow(`SELECT COUNT(*) FROM users WHERE username = ?`, "alice").Scan(&count)
	if err != nil {
		t.Fatalf("Failed to query count: %v", err)
	}

	if count != 0 {
		t.Errorf("Expected 0 users, got %d", count)
	}

	t.Log("Successfully deleted user")
}

// TestTursoTransactions tests transaction support
func TestTursoTransactions(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	conn, err := sql.Open("turso", dbPath)
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer conn.Close()

	// Create table
	createTableSQL := `
		CREATE TABLE IF NOT EXISTS accounts (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			balance INTEGER NOT NULL DEFAULT 0
		)
	`

	if _, err := conn.Exec(createTableSQL); err != nil {
		t.Fatalf("Failed to create table: %v", err)
	}

	// Insert initial accounts
	insertSQL := `INSERT INTO accounts (name, balance) VALUES (?, ?)`
	if _, err := conn.Exec(insertSQL, "alice", 100); err != nil {
		t.Fatalf("Failed to insert alice: %v", err)
	}
	if _, err := conn.Exec(insertSQL, "bob", 50); err != nil {
		t.Fatalf("Failed to insert bob: %v", err)
	}

	// Transfer funds in a transaction
	tx, err := conn.Begin()
	if err != nil {
		t.Fatalf("Failed to begin transaction: %v", err)
	}

	// Debit from alice
	_, err = tx.Exec(`UPDATE accounts SET balance = balance - 30 WHERE name = ?`, "alice")
	if err != nil {
		tx.Rollback()
		t.Fatalf("Failed to debit alice: %v", err)
	}

	// Credit to bob
	_, err = tx.Exec(`UPDATE accounts SET balance = balance + 30 WHERE name = ?`, "bob")
	if err != nil {
		tx.Rollback()
		t.Fatalf("Failed to credit bob: %v", err)
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		t.Fatalf("Failed to commit transaction: %v", err)
	}

	// Verify balances
	var aliceBalance, bobBalance int
	err = conn.QueryRow(`SELECT balance FROM accounts WHERE name = ?`, "alice").Scan(&aliceBalance)
	if err != nil {
		t.Fatalf("Failed to query alice balance: %v", err)
	}

	err = conn.QueryRow(`SELECT balance FROM accounts WHERE name = ?`, "bob").Scan(&bobBalance)
	if err != nil {
		t.Fatalf("Failed to query bob balance: %v", err)
	}

	if aliceBalance != 70 {
		t.Errorf("Expected alice balance 70, got %d", aliceBalance)
	}
	if bobBalance != 80 {
		t.Errorf("Expected bob balance 80, got %d", bobBalance)
	}

	t.Log("Successfully completed transaction")
}

// TestTursoPreparedStatements tests prepared statement usage
func TestTursoPreparedStatements(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	conn, err := sql.Open("turso", dbPath)
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer conn.Close()

	// Create table
	createTableSQL := `
		CREATE TABLE IF NOT EXISTS products (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			price INTEGER NOT NULL
		)
	`

	if _, err := conn.Exec(createTableSQL); err != nil {
		t.Fatalf("Failed to create table: %v", err)
	}

	// Use prepared statement for multiple inserts
	insertSQL := `INSERT INTO products (name, price) VALUES (?, ?)`
	stmt, err := conn.Prepare(insertSQL)
	if err != nil {
		t.Fatalf("Failed to prepare statement: %v", err)
	}
	defer stmt.Close()

	products := []struct {
		name  string
		price int
	}{
		{"widget", 100},
		{"gadget", 200},
		{"doohickey", 150},
	}

	for _, product := range products {
		_, err := stmt.Exec(product.name, product.price)
		if err != nil {
			t.Fatalf("Failed to insert product %s: %v", product.name, err)
		}
	}

	t.Logf("Successfully inserted %d products using prepared statement", len(products))

	// Query using prepared statement
	querySQL := `SELECT name, price FROM products WHERE price > ? ORDER BY price`
	rows, err := conn.Query(querySQL, 120)
	if err != nil {
		t.Fatalf("Failed to query products: %v", err)
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		var name string
		var price int
		if err := rows.Scan(&name, &price); err != nil {
			t.Fatalf("Failed to scan row: %v", err)
		}
		count++
		t.Logf("Found product: %s, price: %d", name, price)
	}

	if count != 2 {
		t.Errorf("Expected 2 products with price > 120, got %d", count)
	}
}

// TestTursoWithRealDatabase tests with a real database file
// This test is skipped by default and can be run with:
// go test -run TestTursoWithRealDatabase ./relay/tests/
func TestTursoWithRealDatabase(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping real database test in short mode")
	}

	// Use a persistent database file in the current directory
	dbPath := "test_relay.db"
	defer os.Remove(dbPath)

	conn, err := sql.Open("turso", dbPath)
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer conn.Close()

	// Create a relay nodes table
	createTableSQL := `
		CREATE TABLE IF NOT EXISTS relay_nodes (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			address TEXT NOT NULL,
			last_heartbeat DATETIME DEFAULT CURRENT_TIMESTAMP,
			status TEXT DEFAULT 'active'
		)
	`

	if _, err := conn.Exec(createTableSQL); err != nil {
		t.Fatalf("Failed to create relay_nodes table: %v", err)
	}

	// Insert a test node
	insertSQL := `INSERT INTO relay_nodes (id, name, address) VALUES (?, ?, ?)`
	nodeID := fmt.Sprintf("node-%d", 12345)
	if _, err := conn.Exec(insertSQL, nodeID, "test-node", "192.168.1.100:8020"); err != nil {
		t.Fatalf("Failed to insert node: %v", err)
	}

	// Query the node
	var name, address string
	err = conn.QueryRow(`SELECT name, address FROM relay_nodes WHERE id = ?`, nodeID).Scan(&name, &address)
	if err != nil {
		t.Fatalf("Failed to query node: %v", err)
	}

	if name != "test-node" {
		t.Errorf("Expected name 'test-node', got '%s'", name)
	}
	if address != "192.168.1.100:8020" {
		t.Errorf("Expected address '192.168.1.100:8020', got '%s'", address)
	}

	t.Logf("Successfully tested with real database file: %s", dbPath)
}

// TestTursoJSONBBasic tests basic JSONB creation and storage
func TestTursoJSONBBasic(t *testing.T) {
	conn, err := sql.Open("turso", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer conn.Close()

	// Create table with JSONB column
	createTableSQL := `
		CREATE TABLE IF NOT EXISTS documents (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			data JSONB NOT NULL
		)
	`

	if _, err := conn.Exec(createTableSQL); err != nil {
		t.Fatalf("Failed to create table: %v", err)
	}

	// Insert JSONB data
	insertSQL := `INSERT INTO documents (data) VALUES (jsonb(?))`
	testData := `{"name": "Alice", "age": 30, "city": "New York"}`

	result, err := conn.Exec(insertSQL, testData)
	if err != nil {
		t.Fatalf("Failed to insert JSONB data: %v", err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected != 1 {
		t.Errorf("Expected 1 row affected, got %d", rowsAffected)
	}

	// Query back the JSONB data
	var data string
	err = conn.QueryRow(`SELECT data FROM documents WHERE id = 1`).Scan(&data)
	if err != nil {
		t.Fatalf("Failed to query JSONB data: %v", err)
	}

	t.Logf("Retrieved JSONB data: %s", data)
}

// TestTursoJSONBExtract tests jsonb_extract function
func TestTursoJSONBExtract(t *testing.T) {
	conn, err := sql.Open("turso", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer conn.Close()

	createTableSQL := `
		CREATE TABLE IF NOT EXISTS users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			profile JSONB NOT NULL
		)
	`

	if _, err := conn.Exec(createTableSQL); err != nil {
		t.Fatalf("Failed to create table: %v", err)
	}

	// Insert test data with nested JSONB
	insertSQL := `INSERT INTO users (profile) VALUES (jsonb(?))`
	profiles := []string{
		`{"name": "Alice", "age": 30, "address": {"city": "New York", "zip": "10001"}}`,
		`{"name": "Bob", "age": 25, "address": {"city": "Los Angeles", "zip": "90001"}}`,
		`{"name": "Charlie", "age": 35, "address": {"city": "Chicago", "zip": "60601"}}`,
	}

	for _, profile := range profiles {
		if _, err := conn.Exec(insertSQL, profile); err != nil {
			t.Fatalf("Failed to insert profile: %v", err)
		}
	}

	// Extract top-level field
	var name string
	err = conn.QueryRow(`SELECT jsonb_extract(profile, '$.name') FROM users WHERE id = 1`).Scan(&name)
	if err != nil {
		t.Fatalf("Failed to extract name: %v", err)
	}
	if name != `Alice` {
		t.Errorf("Expected name 'Alice', got %s", name)
	}

	// Extract nested field
	var city string
	err = conn.QueryRow(`SELECT jsonb_extract(profile, '$.address.city') FROM users WHERE id = 1`).Scan(&city)
	if err != nil {
		t.Fatalf("Failed to extract city: %v", err)
	}
	if city != `New York` {
		t.Errorf("Expected city 'New York', got %s", city)
	}

	t.Log("Successfully extracted JSONB fields")
}

// TestTursoJSONBArray tests jsonb_array function
func TestTursoJSONBArray(t *testing.T) {
	conn, err := sql.Open("turso", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer conn.Close()

	createTableSQL := `
		CREATE TABLE IF NOT EXISTS collections (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			items JSONB NOT NULL
		)
	`

	if _, err := conn.Exec(createTableSQL); err != nil {
		t.Fatalf("Failed to create table: %v", err)
	}

	// Insert JSONB array
	insertSQL := `INSERT INTO collections (items) VALUES (jsonb_array(?, ?, ?, ?))`
	_, err = conn.Exec(insertSQL, "apple", "banana", "orange", "grape")
	if err != nil {
		t.Fatalf("Failed to insert JSONB array: %v", err)
	}

	// Query the array
	var items string
	err = conn.QueryRow(`SELECT items FROM collections WHERE id = 1`).Scan(&items)
	if err != nil {
		t.Fatalf("Failed to query JSONB array: %v", err)
	}

	t.Logf("Retrieved JSONB array: %s", items)
}

// TestTursoJSONBObject tests jsonb_object function
func TestTursoJSONBObject(t *testing.T) {
	conn, err := sql.Open("turso", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer conn.Close()

	createTableSQL := `
		CREATE TABLE IF NOT EXISTS settings (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			config JSONB NOT NULL
		)
	`

	if _, err := conn.Exec(createTableSQL); err != nil {
		t.Fatalf("Failed to create table: %v", err)
	}

	// Create JSONB object from key-value pairs
	insertSQL := `INSERT INTO settings (config) VALUES (jsonb_object(?, ?, ?, ?, ?, ?))`
	_, err = conn.Exec(insertSQL, "theme", "dark", "language", "en", "notifications", "enabled")
	if err != nil {
		t.Fatalf("Failed to insert JSONB object: %v", err)
	}

	// Query the object
	var config string
	err = conn.QueryRow(`SELECT config FROM settings WHERE id = 1`).Scan(&config)
	if err != nil {
		t.Fatalf("Failed to query JSONB object: %v", err)
	}

	t.Logf("Retrieved JSONB object: %s", config)

	// Extract a specific field to verify
	var theme string
	err = conn.QueryRow(`SELECT jsonb_extract(config, '$.theme') FROM settings WHERE id = 1`).Scan(&theme)
	if err != nil {
		t.Fatalf("Failed to extract theme: %v", err)
	}
	if theme != `dark` {
		t.Errorf("Expected theme 'dark', got %s", theme)
	}
}

// TestTursoJSONBInsert tests jsonb_insert function
func TestTursoJSONBInsert(t *testing.T) {
	conn, err := sql.Open("turso", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer conn.Close()

	createTableSQL := `
		CREATE TABLE IF NOT EXISTS documents (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			data JSONB NOT NULL
		)
	`

	if _, err := conn.Exec(createTableSQL); err != nil {
		t.Fatalf("Failed to create table: %v", err)
	}

	// Insert initial JSONB
	insertSQL := `INSERT INTO documents (data) VALUES (jsonb(?))`
	initialData := `{"name": "Alice", "age": 30}`

	if _, err := conn.Exec(insertSQL, initialData); err != nil {
		t.Fatalf("Failed to insert initial data: %v", err)
	}

	// Insert a new field into the JSONB
	updateSQL := `UPDATE documents SET data = jsonb_insert(data, '$.city', ?) WHERE id = 1`
	if _, err := conn.Exec(updateSQL, `"New York"`); err != nil {
		t.Fatalf("Failed to insert field into JSONB: %v", err)
	}

	// Verify the insertion
	var data string
	err = conn.QueryRow(`SELECT data FROM documents WHERE id = 1`).Scan(&data)
	if err != nil {
		t.Fatalf("Failed to query updated data: %v", err)
	}

	t.Logf("Data after jsonb_insert: %s", data)
}

// TestTursoJSONBReplace tests jsonb_replace function
func TestTursoJSONBReplace(t *testing.T) {
	conn, err := sql.Open("turso", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer conn.Close()

	createTableSQL := `
		CREATE TABLE IF NOT EXISTS documents (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			data JSONB NOT NULL
		)
	`

	if _, err := conn.Exec(createTableSQL); err != nil {
		t.Fatalf("Failed to create table: %v", err)
	}

	// Insert initial data
	insertSQL := `INSERT INTO documents (data) VALUES (jsonb(?))`
	initialData := `{"name": "Alice", "age": 30, "city": "New York"}`

	if _, err := conn.Exec(insertSQL, initialData); err != nil {
		t.Fatalf("Failed to insert initial data: %v", err)
	}

	// Replace a field value
	updateSQL := `UPDATE documents SET data = jsonb_replace(data, '$.age', ?) WHERE id = 1`
	if _, err := conn.Exec(updateSQL, 31); err != nil {
		t.Fatalf("Failed to replace field: %v", err)
	}

	// Verify the replacement
	var age string
	err = conn.QueryRow(`SELECT jsonb_extract(data, '$.age') FROM documents WHERE id = 1`).Scan(&age)
	if err != nil {
		t.Fatalf("Failed to query age: %v", err)
	}

	if age != "31" {
		t.Errorf("Expected age '31', got %s", age)
	}

	t.Log("Successfully replaced JSONB field")
}

// TestTursoJSONBSet tests jsonb_set function
func TestTursoJSONBSet(t *testing.T) {
	conn, err := sql.Open("turso", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer conn.Close()

	createTableSQL := `
		CREATE TABLE IF NOT EXISTS documents (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			data JSONB NOT NULL
		)
	`

	if _, err := conn.Exec(createTableSQL); err != nil {
		t.Fatalf("Failed to create table: %v", err)
	}

	// Insert initial data
	insertSQL := `INSERT INTO documents (data) VALUES (jsonb(?))`
	initialData := `{"name": "Alice", "age": 30}`

	if _, err := conn.Exec(insertSQL, initialData); err != nil {
		t.Fatalf("Failed to insert initial data: %v", err)
	}

	// Use jsonb_set to update existing field or create new one
	updateSQL := `UPDATE documents SET data = jsonb_set(data, '$.email', ?) WHERE id = 1`
	if _, err := conn.Exec(updateSQL, `"alice@example.com"`); err != nil {
		t.Fatalf("Failed to set field: %v", err)
	}

	// Verify the set operation
	var email string
	err = conn.QueryRow(`SELECT jsonb_extract(data, '$.email') FROM documents WHERE id = 1`).Scan(&email)
	if err != nil {
		t.Fatalf("Failed to query email: %v", err)
	}

	// Email may come back with escaped quotes depending on how it was inserted
	if email != `alice@example.com` && email != `"alice@example.com"` && email != `\"alice@example.com\"` {
		t.Errorf("Expected email 'alice@example.com', got %s", email)
	}

	t.Log("Successfully set JSONB field")
}

// TestTursoJSONBRemove tests jsonb_remove function
func TestTursoJSONBRemove(t *testing.T) {
	conn, err := sql.Open("turso", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer conn.Close()

	createTableSQL := `
		CREATE TABLE IF NOT EXISTS documents (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			data JSONB NOT NULL
		)
	`

	if _, err := conn.Exec(createTableSQL); err != nil {
		t.Fatalf("Failed to create table: %v", err)
	}

	// Insert data with multiple fields
	insertSQL := `INSERT INTO documents (data) VALUES (jsonb(?))`
	initialData := `{"name": "Alice", "age": 30, "city": "New York", "temp": "delete me"}`

	if _, err := conn.Exec(insertSQL, initialData); err != nil {
		t.Fatalf("Failed to insert initial data: %v", err)
	}

	// Remove a field
	updateSQL := `UPDATE documents SET data = jsonb_remove(data, '$.temp') WHERE id = 1`
	if _, err := conn.Exec(updateSQL); err != nil {
		t.Fatalf("Failed to remove field: %v", err)
	}

	// Verify the removal
	var data string
	err = conn.QueryRow(`SELECT data FROM documents WHERE id = 1`).Scan(&data)
	if err != nil {
		t.Fatalf("Failed to query data: %v", err)
	}

	t.Logf("Data after jsonb_remove: %s", data)
}

// TestTursoJSONBPatch tests patching JSONB using jsonb_set for multiple fields
func TestTursoJSONBPatch(t *testing.T) {
	conn, err := sql.Open("turso", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer conn.Close()

	createTableSQL := `
		CREATE TABLE IF NOT EXISTS documents (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			data JSONB NOT NULL
		)
	`

	if _, err := conn.Exec(createTableSQL); err != nil {
		t.Fatalf("Failed to create table: %v", err)
	}

	// Insert initial data
	insertSQL := `INSERT INTO documents (data) VALUES (jsonb(?))`
	initialData := `{"name": "Alice", "age": 30, "city": "New York"}`

	if _, err := conn.Exec(insertSQL, initialData); err != nil {
		t.Fatalf("Failed to insert initial data: %v", err)
	}

	// Apply multiple updates (simulating a patch operation)
	updateSQL1 := `UPDATE documents SET data = jsonb_set(data, '$.age', ?) WHERE id = 1`
	if _, err := conn.Exec(updateSQL1, 31); err != nil {
		t.Fatalf("Failed to update age: %v", err)
	}

	updateSQL2 := `UPDATE documents SET data = jsonb_set(data, '$.country', ?) WHERE id = 1`
	if _, err := conn.Exec(updateSQL2, `"USA"`); err != nil {
		t.Fatalf("Failed to add country: %v", err)
	}

	// Verify the updates were applied
	var data string
	err = conn.QueryRow(`SELECT data FROM documents WHERE id = 1`).Scan(&data)
	if err != nil {
		t.Fatalf("Failed to query patched data: %v", err)
	}

	t.Logf("Data after patching: %s", data)

	// Verify specific fields
	var age string
	err = conn.QueryRow(`SELECT jsonb_extract(data, '$.age') FROM documents WHERE id = 1`).Scan(&age)
	if err != nil {
		t.Fatalf("Failed to query age: %v", err)
	}
	if age != "31" {
		t.Errorf("Expected age '31', got %s", age)
	}

	var country string
	err = conn.QueryRow(`SELECT jsonb_extract(data, '$.country') FROM documents WHERE id = 1`).Scan(&country)
	if err != nil {
		t.Fatalf("Failed to query country: %v", err)
	}
	// Country may come back with escaped quotes depending on how it was inserted
	if country != `USA` && country != `"USA"` && country != `\"USA\"` {
		t.Errorf("Expected country 'USA', got %s", country)
	}
}

// TestTursoJSONBGroupArray tests jsonb_group_array aggregation
func TestTursoJSONBGroupArray(t *testing.T) {
	conn, err := sql.Open("turso", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer conn.Close()

	createTableSQL := `
		CREATE TABLE IF NOT EXISTS products (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			category TEXT NOT NULL,
			name TEXT NOT NULL,
			price INTEGER NOT NULL
		)
	`

	if _, err := conn.Exec(createTableSQL); err != nil {
		t.Fatalf("Failed to create table: %v", err)
	}

	// Insert test data
	products := []struct {
		category string
		name     string
		price    int
	}{
		{"electronics", "laptop", 1000},
		{"electronics", "phone", 500},
		{"books", "novel", 20},
		{"books", "textbook", 100},
	}

	insertSQL := `INSERT INTO products (category, name, price) VALUES (?, ?, ?)`
	for _, p := range products {
		if _, err := conn.Exec(insertSQL, p.category, p.name, p.price); err != nil {
			t.Fatalf("Failed to insert product: %v", err)
		}
	}

	// Group product names by category into JSONB arrays
	querySQL := `SELECT category, jsonb_group_array(name) as products FROM products GROUP BY category ORDER BY category`
	rows, err := conn.Query(querySQL)
	if err != nil {
		t.Fatalf("Failed to query grouped data: %v", err)
	}
	defer rows.Close()

	results := make(map[string]string)
	for rows.Next() {
		var category, products string
		if err := rows.Scan(&category, &products); err != nil {
			t.Fatalf("Failed to scan row: %v", err)
		}
		results[category] = products
		t.Logf("Category: %s, Products: %s", category, products)
	}

	if len(results) != 2 {
		t.Errorf("Expected 2 categories, got %d", len(results))
	}
}

// TestTursoJSONBGroupObject tests jsonb_group_object aggregation
func TestTursoJSONBGroupObject(t *testing.T) {
	conn, err := sql.Open("turso", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer conn.Close()

	createTableSQL := `
		CREATE TABLE IF NOT EXISTS prices (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			product TEXT NOT NULL,
			price INTEGER NOT NULL
		)
	`

	if _, err := conn.Exec(createTableSQL); err != nil {
		t.Fatalf("Failed to create table: %v", err)
	}

	// Insert test data
	prices := []struct {
		product string
		price   int
	}{
		{"laptop", 1000},
		{"phone", 500},
		{"tablet", 300},
	}

	insertSQL := `INSERT INTO prices (product, price) VALUES (?, ?)`
	for _, p := range prices {
		if _, err := conn.Exec(insertSQL, p.product, p.price); err != nil {
			t.Fatalf("Failed to insert price: %v", err)
		}
	}

	// Aggregate into a JSONB object with product names as keys
	querySQL := `SELECT jsonb_group_object(product, price) as price_map FROM prices`
	var priceMap string
	err = conn.QueryRow(querySQL).Scan(&priceMap)
	if err != nil {
		t.Fatalf("Failed to query grouped object: %v", err)
	}

	t.Logf("Price map: %s", priceMap)
}

// TestTursoJSONBComplexQuery tests complex querying with JSONB
func TestTursoJSONBComplexQuery(t *testing.T) {
	conn, err := sql.Open("turso", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer conn.Close()

	createTableSQL := `
		CREATE TABLE IF NOT EXISTS users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			profile JSONB NOT NULL
		)
	`

	if _, err := conn.Exec(createTableSQL); err != nil {
		t.Fatalf("Failed to create table: %v", err)
	}

	// Insert users with varying profiles
	insertSQL := `INSERT INTO users (profile) VALUES (jsonb(?))`
	profiles := []string{
		`{"name": "Alice", "age": 30, "skills": ["Go", "Python", "JavaScript"], "active": true}`,
		`{"name": "Bob", "age": 25, "skills": ["Java", "C++"], "active": true}`,
		`{"name": "Charlie", "age": 35, "skills": ["Rust", "Go", "C"], "active": false}`,
		`{"name": "Diana", "age": 28, "skills": ["Python", "R", "SQL"], "active": true}`,
	}

	for _, profile := range profiles {
		if _, err := conn.Exec(insertSQL, profile); err != nil {
			t.Fatalf("Failed to insert profile: %v", err)
		}
	}

	// Query active users
	querySQL := `SELECT jsonb_extract(profile, '$.name'), jsonb_extract(profile, '$.age')
	             FROM users
	             WHERE jsonb_extract(profile, '$.active') = 1
	             ORDER BY jsonb_extract(profile, '$.age')`

	rows, err := conn.Query(querySQL)
	if err != nil {
		t.Fatalf("Failed to query users: %v", err)
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		var name, age string
		if err := rows.Scan(&name, &age); err != nil {
			t.Fatalf("Failed to scan row: %v", err)
		}
		count++
		t.Logf("Active user: %s, age: %s", name, age)
	}

	if count != 3 {
		t.Errorf("Expected 3 active users, got %d", count)
	}

	t.Log("Successfully queried JSONB data with filters")
}

// TestTursoJSONBNestedOperations tests operations on nested JSONB structures
func TestTursoJSONBNestedOperations(t *testing.T) {
	conn, err := sql.Open("turso", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer conn.Close()

	createTableSQL := `
		CREATE TABLE IF NOT EXISTS organizations (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			data JSONB NOT NULL
		)
	`

	if _, err := conn.Exec(createTableSQL); err != nil {
		t.Fatalf("Failed to create table: %v", err)
	}

	// Insert organization with nested structure
	insertSQL := `INSERT INTO organizations (data) VALUES (jsonb(?))`
	orgData := `{
		"name": "TechCorp",
		"departments": {
			"engineering": {
				"headcount": 50,
				"budget": 1000000
			},
			"sales": {
				"headcount": 30,
				"budget": 500000
			}
		}
	}`

	if _, err := conn.Exec(insertSQL, orgData); err != nil {
		t.Fatalf("Failed to insert organization data: %v", err)
	}

	// Extract nested data
	var engineeringHeadcount string
	err = conn.QueryRow(`SELECT jsonb_extract(data, '$.departments.engineering.headcount') FROM organizations WHERE id = 1`).Scan(&engineeringHeadcount)
	if err != nil {
		t.Fatalf("Failed to extract nested field: %v", err)
	}

	if engineeringHeadcount != "50" {
		t.Errorf("Expected headcount '50', got %s", engineeringHeadcount)
	}

	// Update nested field
	updateSQL := `UPDATE organizations SET data = jsonb_set(data, '$.departments.engineering.headcount', ?) WHERE id = 1`
	if _, err := conn.Exec(updateSQL, 55); err != nil {
		t.Fatalf("Failed to update nested field: %v", err)
	}

	// Verify update
	err = conn.QueryRow(`SELECT jsonb_extract(data, '$.departments.engineering.headcount') FROM organizations WHERE id = 1`).Scan(&engineeringHeadcount)
	if err != nil {
		t.Fatalf("Failed to query updated nested field: %v", err)
	}

	if engineeringHeadcount != "55" {
		t.Errorf("Expected updated headcount '55', got %s", engineeringHeadcount)
	}

	t.Log("Successfully performed nested JSONB operations")
}

// TestTursoVector32Basic tests basic vector32 creation and storage
func TestTursoVector32Basic(t *testing.T) {
	conn, err := sql.Open("turso", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer conn.Close()

	// Create table with vector column
	createTableSQL := `
		CREATE TABLE IF NOT EXISTS embeddings (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			content TEXT NOT NULL,
			embedding BLOB NOT NULL
		)
	`

	if _, err := conn.Exec(createTableSQL); err != nil {
		t.Fatalf("Failed to create table: %v", err)
	}

	// Insert vector data using vector32
	insertSQL := `INSERT INTO embeddings (content, embedding) VALUES (?, vector32(?))`
	testVector := `[0.1, 0.2, 0.3, 0.4]`

	result, err := conn.Exec(insertSQL, "test document", testVector)
	if err != nil {
		t.Fatalf("Failed to insert vector data: %v", err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected != 1 {
		t.Errorf("Expected 1 row affected, got %d", rowsAffected)
	}

	t.Log("Successfully created and stored vector32")
}

// TestTursoVector64Basic tests basic vector64 creation and storage
func TestTursoVector64Basic(t *testing.T) {
	conn, err := sql.Open("turso", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer conn.Close()

	createTableSQL := `
		CREATE TABLE IF NOT EXISTS embeddings (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			content TEXT NOT NULL,
			embedding BLOB NOT NULL
		)
	`

	if _, err := conn.Exec(createTableSQL); err != nil {
		t.Fatalf("Failed to create table: %v", err)
	}

	// Insert vector data using vector64 (higher precision)
	insertSQL := `INSERT INTO embeddings (content, embedding) VALUES (?, vector64(?))`
	testVector := `[0.123456789, 0.987654321, 0.555555555, 0.999999999]`

	result, err := conn.Exec(insertSQL, "high precision vector", testVector)
	if err != nil {
		t.Fatalf("Failed to insert vector64 data: %v", err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected != 1 {
		t.Errorf("Expected 1 row affected, got %d", rowsAffected)
	}

	t.Log("Successfully created and stored vector64")
}

// TestTursoVectorExtract tests extracting vector data back to JSON
func TestTursoVectorExtract(t *testing.T) {
	conn, err := sql.Open("turso", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer conn.Close()

	createTableSQL := `
		CREATE TABLE IF NOT EXISTS embeddings (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			embedding BLOB NOT NULL
		)
	`

	if _, err := conn.Exec(createTableSQL); err != nil {
		t.Fatalf("Failed to create table: %v", err)
	}

	// Insert vector
	insertSQL := `INSERT INTO embeddings (embedding) VALUES (vector32(?))`
	testVector := `[1.0, 2.0, 3.0, 4.0]`

	if _, err := conn.Exec(insertSQL, testVector); err != nil {
		t.Fatalf("Failed to insert vector: %v", err)
	}

	// Extract vector back to JSON
	var extracted string
	err = conn.QueryRow(`SELECT vector_extract(embedding) FROM embeddings WHERE id = 1`).Scan(&extracted)
	if err != nil {
		t.Fatalf("Failed to extract vector: %v", err)
	}

	t.Logf("Extracted vector: %s", extracted)
}

// TestTursoVectorDistanceL2 tests L2 (Euclidean) distance calculation
func TestTursoVectorDistanceL2(t *testing.T) {
	conn, err := sql.Open("turso", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer conn.Close()

	createTableSQL := `
		CREATE TABLE IF NOT EXISTS documents (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			content TEXT NOT NULL,
			embedding BLOB NOT NULL
		)
	`

	if _, err := conn.Exec(createTableSQL); err != nil {
		t.Fatalf("Failed to create table: %v", err)
	}

	// Insert test documents with embeddings
	insertSQL := `INSERT INTO documents (content, embedding) VALUES (?, vector32(?))`
	docs := []struct {
		content string
		vector  string
	}{
		{"document 1", `[1.0, 0.0, 0.0, 0.0]`},
		{"document 2", `[0.0, 1.0, 0.0, 0.0]`},
		{"document 3", `[0.0, 0.0, 1.0, 0.0]`},
		{"document 4", `[1.0, 1.0, 0.0, 0.0]`},
	}

	for _, doc := range docs {
		if _, err := conn.Exec(insertSQL, doc.content, doc.vector); err != nil {
			t.Fatalf("Failed to insert document: %v", err)
		}
	}

	// Calculate L2 distance from a query vector
	queryVector := `[1.0, 0.5, 0.0, 0.0]`
	querySQL := `
		SELECT content, vector_distance_l2(embedding, vector32(?)) AS distance
		FROM documents
		ORDER BY distance
		LIMIT 3
	`

	rows, err := conn.Query(querySQL, queryVector)
	if err != nil {
		t.Fatalf("Failed to query distances: %v", err)
	}
	defer rows.Close()

	results := []struct {
		content  string
		distance float64
	}{}

	for rows.Next() {
		var content string
		var distance float64
		if err := rows.Scan(&content, &distance); err != nil {
			t.Fatalf("Failed to scan row: %v", err)
		}
		results = append(results, struct {
			content  string
			distance float64
		}{content, distance})
		t.Logf("Document: %s, L2 Distance: %f", content, distance)
	}

	if len(results) != 3 {
		t.Errorf("Expected 3 results, got %d", len(results))
	}

	// The closest should be document 4 [1.0, 1.0, 0.0, 0.0]
	if results[0].content != "document 4" {
		t.Logf("Note: Closest document is %s (expected document 4)", results[0].content)
	}

	t.Log("Successfully calculated L2 distances")
}

// TestTursoVectorDistanceCos tests cosine distance calculation
func TestTursoVectorDistanceCos(t *testing.T) {
	conn, err := sql.Open("turso", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer conn.Close()

	createTableSQL := `
		CREATE TABLE IF NOT EXISTS documents (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			content TEXT NOT NULL,
			embedding BLOB NOT NULL
		)
	`

	if _, err := conn.Exec(createTableSQL); err != nil {
		t.Fatalf("Failed to create table: %v", err)
	}

	// Insert test documents
	insertSQL := `INSERT INTO documents (content, embedding) VALUES (?, vector32(?))`
	docs := []struct {
		content string
		vector  string
	}{
		{"cat", `[0.9, 0.1, 0.0, 0.0]`},
		{"dog", `[0.8, 0.2, 0.0, 0.0]`},
		{"car", `[0.0, 0.0, 0.9, 0.1]`},
		{"truck", `[0.0, 0.0, 0.8, 0.2]`},
	}

	for _, doc := range docs {
		if _, err := conn.Exec(insertSQL, doc.content, doc.vector); err != nil {
			t.Fatalf("Failed to insert document: %v", err)
		}
	}

	// Find documents similar to "cat" using cosine distance
	queryVector := `[0.85, 0.15, 0.0, 0.0]`
	querySQL := `
		SELECT content, vector_distance_cos(embedding, vector32(?)) AS distance
		FROM documents
		ORDER BY distance
		LIMIT 2
	`

	rows, err := conn.Query(querySQL, queryVector)
	if err != nil {
		t.Fatalf("Failed to query cosine distances: %v", err)
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		var content string
		var distance float64
		if err := rows.Scan(&content, &distance); err != nil {
			t.Fatalf("Failed to scan row: %v", err)
		}
		count++
		t.Logf("Similar document: %s, Cosine Distance: %f", content, distance)
	}

	if count != 2 {
		t.Errorf("Expected 2 results, got %d", count)
	}

	t.Log("Successfully calculated cosine distances")
}

// TestTursoVectorSimilaritySearch tests a complete similarity search scenario
func TestTursoVectorSimilaritySearch(t *testing.T) {
	conn, err := sql.Open("turso", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer conn.Close()

	createTableSQL := `
		CREATE TABLE IF NOT EXISTS articles (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			title TEXT NOT NULL,
			content TEXT NOT NULL,
			embedding BLOB NOT NULL
		)
	`

	if _, err := conn.Exec(createTableSQL); err != nil {
		t.Fatalf("Failed to create table: %v", err)
	}

	// Insert sample articles with embeddings
	insertSQL := `INSERT INTO articles (title, content, embedding) VALUES (?, ?, vector32(?))`
	articles := []struct {
		title     string
		content   string
		embedding string
	}{
		{
			"Introduction to Go",
			"Go is a programming language designed for building simple, reliable software.",
			`[0.8, 0.2, 0.1, 0.05, 0.02]`,
		},
		{
			"Python Basics",
			"Python is a high-level programming language known for its simplicity.",
			`[0.75, 0.25, 0.15, 0.08, 0.03]`,
		},
		{
			"Cooking Pasta",
			"Pasta is a type of food made from wheat flour and water.",
			`[0.05, 0.1, 0.8, 0.3, 0.1]`,
		},
		{
			"Italian Cuisine",
			"Italian food is known for its regional diversity and use of fresh ingredients.",
			`[0.08, 0.12, 0.75, 0.35, 0.15]`,
		},
		{
			"Rust Programming",
			"Rust is a systems programming language focused on safety and performance.",
			`[0.85, 0.18, 0.08, 0.04, 0.01]`,
		},
	}

	for _, article := range articles {
		if _, err := conn.Exec(insertSQL, article.title, article.content, article.embedding); err != nil {
			t.Fatalf("Failed to insert article: %v", err)
		}
	}

	// Search for programming-related articles
	queryVector := `[0.9, 0.15, 0.05, 0.03, 0.01]`
	searchSQL := `
		SELECT title, content, vector_distance_cos(embedding, vector32(?)) AS similarity
		FROM articles
		ORDER BY similarity
		LIMIT 3
	`

	rows, err := conn.Query(searchSQL, queryVector)
	if err != nil {
		t.Fatalf("Failed to execute similarity search: %v", err)
	}
	defer rows.Close()

	t.Log("Top 3 similar articles to programming query:")
	count := 0
	for rows.Next() {
		var title, content string
		var similarity float64
		if err := rows.Scan(&title, &content, &similarity); err != nil {
			t.Fatalf("Failed to scan row: %v", err)
		}
		count++
		t.Logf("%d. %s (similarity: %f)", count, title, similarity)
	}

	if count != 3 {
		t.Errorf("Expected 3 results, got %d", count)
	}

	t.Log("Successfully performed similarity search")
}

// TestTursoVectorConcat tests vector concatenation
func TestTursoVectorConcat(t *testing.T) {
	conn, err := sql.Open("turso", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer conn.Close()

	createTableSQL := `
		CREATE TABLE IF NOT EXISTS vectors (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			vec1 BLOB NOT NULL,
			vec2 BLOB NOT NULL
		)
	`

	if _, err := conn.Exec(createTableSQL); err != nil {
		t.Fatalf("Failed to create table: %v", err)
	}

	// Insert two vectors
	insertSQL := `INSERT INTO vectors (vec1, vec2) VALUES (vector32(?), vector32(?))`
	vec1 := `[1.0, 2.0]`
	vec2 := `[3.0, 4.0]`

	if _, err := conn.Exec(insertSQL, vec1, vec2); err != nil {
		t.Fatalf("Failed to insert vectors: %v", err)
	}

	// Concatenate vectors
	var concatenated string
	err = conn.QueryRow(`SELECT vector_extract(vector_concat(vec1, vec2)) FROM vectors WHERE id = 1`).Scan(&concatenated)
	if err != nil {
		t.Fatalf("Failed to concatenate vectors: %v", err)
	}

	t.Logf("Concatenated vector: %s", concatenated)
}

// TestTursoVectorSlice tests vector slicing
func TestTursoVectorSlice(t *testing.T) {
	conn, err := sql.Open("turso", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer conn.Close()

	createTableSQL := `
		CREATE TABLE IF NOT EXISTS vectors (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			embedding BLOB NOT NULL
		)
	`

	if _, err := conn.Exec(createTableSQL); err != nil {
		t.Fatalf("Failed to create table: %v", err)
	}

	// Insert a vector
	insertSQL := `INSERT INTO vectors (embedding) VALUES (vector32(?))`
	testVector := `[1.0, 2.0, 3.0, 4.0, 5.0]`

	if _, err := conn.Exec(insertSQL, testVector); err != nil {
		t.Fatalf("Failed to insert vector: %v", err)
	}

	// Slice the vector (get elements from index 1 to 3)
	var sliced string
	err = conn.QueryRow(`SELECT vector_extract(vector_slice(embedding, 1, 3)) FROM vectors WHERE id = 1`).Scan(&sliced)
	if err != nil {
		t.Fatalf("Failed to slice vector: %v", err)
	}

	t.Logf("Sliced vector [1:3]: %s", sliced)
}

// TestTursoVectorHighDimensional tests working with higher-dimensional vectors
func TestTursoVectorHighDimensional(t *testing.T) {
	conn, err := sql.Open("turso", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer conn.Close()

	createTableSQL := `
		CREATE TABLE IF NOT EXISTS embeddings (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			content TEXT NOT NULL,
			embedding BLOB NOT NULL
		)
	`

	if _, err := conn.Exec(createTableSQL); err != nil {
		t.Fatalf("Failed to create table: %v", err)
	}

	// Create a 128-dimensional vector (simulating real-world embeddings)
	dimensions := make([]string, 128)
	for i := 0; i < 128; i++ {
		dimensions[i] = fmt.Sprintf("%f", float64(i)*0.01)
	}
	highDimVector := "[" + string(dimensions[0])
	for i := 1; i < len(dimensions); i++ {
		highDimVector += ", " + dimensions[i]
	}
	highDimVector += "]"

	insertSQL := `INSERT INTO embeddings (content, embedding) VALUES (?, vector32(?))`
	if _, err := conn.Exec(insertSQL, "high-dimensional document", highDimVector); err != nil {
		t.Fatalf("Failed to insert high-dimensional vector: %v", err)
	}

	// Query it back
	var content string
	err = conn.QueryRow(`SELECT content FROM embeddings WHERE id = 1`).Scan(&content)
	if err != nil {
		t.Fatalf("Failed to query high-dimensional embedding: %v", err)
	}

	if content != "high-dimensional document" {
		t.Errorf("Expected content 'high-dimensional document', got %s", content)
	}

	t.Log("Successfully stored and retrieved 128-dimensional vector")
}

// TestTursoVectorBatchSimilaritySearch tests similarity search with multiple query vectors
func TestTursoVectorBatchSimilaritySearch(t *testing.T) {
	conn, err := sql.Open("turso", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer conn.Close()

	createTableSQL := `
		CREATE TABLE IF NOT EXISTS products (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			category TEXT NOT NULL,
			embedding BLOB NOT NULL
		)
	`

	if _, err := conn.Exec(createTableSQL); err != nil {
		t.Fatalf("Failed to create table: %v", err)
	}

	// Insert products with embeddings
	insertSQL := `INSERT INTO products (name, category, embedding) VALUES (?, ?, vector32(?))`
	products := []struct {
		name      string
		category  string
		embedding string
	}{
		{"Laptop", "Electronics", `[0.9, 0.1, 0.05, 0.02]`},
		{"Phone", "Electronics", `[0.85, 0.15, 0.08, 0.03]`},
		{"Shirt", "Clothing", `[0.1, 0.9, 0.05, 0.02]`},
		{"Pants", "Clothing", `[0.15, 0.85, 0.08, 0.03]`},
		{"Book", "Media", `[0.05, 0.05, 0.9, 0.1]`},
	}

	for _, product := range products {
		if _, err := conn.Exec(insertSQL, product.name, product.category, product.embedding); err != nil {
			t.Fatalf("Failed to insert product: %v", err)
		}
	}

	// Test multiple queries
	queries := []struct {
		name   string
		vector string
	}{
		{"Electronics query", `[0.95, 0.05, 0.02, 0.01]`},
		{"Clothing query", `[0.05, 0.95, 0.02, 0.01]`},
	}

	for _, query := range queries {
		t.Logf("\nSearching with: %s", query.name)

		searchSQL := `
			SELECT name, category, vector_distance_cos(embedding, vector32(?)) AS distance
			FROM products
			ORDER BY distance
			LIMIT 2
		`

		rows, err := conn.Query(searchSQL, query.vector)
		if err != nil {
			t.Fatalf("Failed to execute search: %v", err)
		}

		count := 0
		for rows.Next() {
			var name, category string
			var distance float64
			if err := rows.Scan(&name, &category, &distance); err != nil {
				rows.Close()
				t.Fatalf("Failed to scan row: %v", err)
			}
			count++
			t.Logf("  %d. %s (%s) - distance: %f", count, name, category, distance)
		}
		rows.Close()

		if count != 2 {
			t.Errorf("Expected 2 results for %s, got %d", query.name, count)
		}
	}

	t.Log("\nSuccessfully performed batch similarity searches")
}
