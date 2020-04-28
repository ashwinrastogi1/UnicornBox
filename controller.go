// Main logic/functionality for the web application.
// This is where you need to implement your own server.
package main

// Reminder that you're not allowed to import anything that isn't part of the Go standard library.
// This includes golang.org/x/
import (
	"database/sql"
	"fmt"
	"io/ioutil"
	_ "io/ioutil"
	"net/http"
	"os"
	_ "os"
	"path/filepath"
	_ "path/filepath"
	"strings"
	"text/template"
	"time"

	log "github.com/sirupsen/logrus"
)

func processRegistration(response http.ResponseWriter, request *http.Request) {
	username := request.FormValue("username")
	password := request.FormValue("password")

	// Check if username already exists
	row := db.QueryRow("SELECT username FROM users WHERE username = ?", username)
	var savedUsername string
	err := row.Scan(&savedUsername)
	if err != sql.ErrNoRows {
		response.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(response, "username %s already exists", savedUsername)
		return
	}

	// Generate salt
	const saltSizeBytes = 16
	salt, err := randomByteString(saltSizeBytes)
	if err != nil {
		response.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(response, err.Error())
		return
	}

	hashedPassword := hashPassword(password, salt)

	_, err = db.Exec("INSERT INTO users VALUES (NULL, ?, ?, ?)", username, hashedPassword, salt)

	if err != nil {
		response.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(response, err.Error())
		return
	}

	// Set a new session cookie
	initSession(response, username)

	// Redirect to next page
	http.Redirect(response, request, "/", http.StatusFound)
}

func processLoginAttempt(response http.ResponseWriter, request *http.Request) {
	// Retrieve submitted values
	username := request.FormValue("username")
	password := request.FormValue("password")

	row := db.QueryRow("SELECT password, salt FROM users WHERE username = ?", username)

	// Parse database response: check for no response or get values
	var encodedHash, encodedSalt string
	err := row.Scan(&encodedHash, &encodedSalt)
	if err == sql.ErrNoRows {
		response.WriteHeader(http.StatusNotFound)
		fmt.Fprintf(response, "unknown user")
		return
	} else if err != nil {
		response.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(response, err.Error())
		return
	}

	// Hash submitted password with salt to allow for comparison
	submittedPassword := hashPassword(password, encodedSalt)

	// Verify password
	if submittedPassword != encodedHash {
		fmt.Fprintf(response, "incorrect password")
		return
	}

	// Set a new session cookie
	initSession(response, username)

	// Redirect to next page
	http.Redirect(response, request, "/", http.StatusFound)
}

func processLogout(response http.ResponseWriter, request *http.Request) {
	// get the session token cookie
	cookie, err := request.Cookie("session_token")

	// empty assignment to suppress unused variable warning
	_, _ = cookie, err

	// get username of currently logged in user
	username := getUsernameFromCtx(request)
	// empty assignment to suppress unused variable warning
	_ = username

	//////////////////////////////////
	// BEGIN TASK 2: YOUR CODE HERE
	//////////////////////////////////
	if username == "" {
		response.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(response, err.Error())
		return
	}

	// TODO: clear the session token cookie in the user's browser
	// HINT: to clear a cookie, set its MaxAge to -1
	http.SetCookie(response, &http.Cookie{
		Name:     cookie.Name,
		Value:    cookie.Value,
		Expires:  cookie.Expires,
		SameSite: cookie.SameSite,
		MaxAge:   -1,
	})

	// TODO: delete the session from the database
	_, err = db.Exec("DELETE FROM sessions WHERE username = ? AND token = ?", username, cookie.Value)
	if err != nil {
		response.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(response, err.Error())
		return
	}

	//////////////////////////////////
	// END TASK 2: YOUR CODE HERE
	//////////////////////////////////

	// redirect to the homepage
	http.Redirect(response, request, "/", http.StatusSeeOther)
}

func processUpload(response http.ResponseWriter, request *http.Request, username string) {

	//////////////////////////////////
	// BEGIN TASK 3: YOUR CODE HERE
	//////////////////////////////////

	// HINT: files should be stored in const filePath = "./files"
	const filePath = "./files"

	// Retrieve file from request
	postFile, fileHeader, formErr := request.FormFile("file")
	_ = postFile

	if formErr != nil { // Error retrieving uploaded file
		response.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(response, formErr.Error())
		return
	}

	// Read content from object
	content, readErr := ioutil.ReadAll(postFile)
	_ = content

	if readErr != nil {
		log.Fatal(readErr)
		return
	}

	// Close file reader
	defer postFile.Close()

	// Store files in data base
	path := filepath.Join(filePath, username, fileHeader.Filename)
	db.Exec("INSERT INTO files VALUES (NULL, ?, ?, ?, ?)", username, path, fileHeader.Filename, "")

	// Store file in disk
	if _, mkdirErr := os.Stat(filepath.Join(filePath, username)); os.IsNotExist(mkdirErr) {
		dirErr := os.Mkdir(filepath.Join(filePath, username), 0700)
		if dirErr != nil {
			response.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(response, dirErr.Error())
			return
		}
	}

	storeErr := ioutil.WriteFile(path, content, 0644)
	if storeErr != nil {
		response.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(response, storeErr.Error())
		return
	}

	//////////////////////////////////
	// END TASK 3: YOUR CODE HERE
	//////////////////////////////////
}

// fileInfo helps you pass information to the template
type fileInfo struct {
	Filename  string
	FileOwner string
	FilePath  string
}

func listFiles(response http.ResponseWriter, request *http.Request, username string) {
	files := make([]fileInfo, 0)

	//////////////////////////////////
	// BEGIN TASK 4: YOUR CODE HERE
	//////////////////////////////////

	// TODO: for each of the user's files, add a
	// corresponding fileInfo struct to the files slice.
	query := "SELECT * FROM files WHERE owner = ? OR shared LIKE " + "'%" + username + " %'"
	rows, err := db.Query(query, username)
	if err != nil {
		response.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(response, err.Error())
		return
	}

	defer rows.Close()

	for rows.Next() {
		var id int
		var shared string
		var tempInfo fileInfo

		if err := rows.Scan(&id, &tempInfo.FileOwner, &tempInfo.FilePath, &tempInfo.Filename, &shared); err != nil {
			response.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(response, err.Error())
			return
		}

		files = append(files, tempInfo)
	}

	// replace this line

	//////////////////////////////////
	// END TASK 4: YOUR CODE HERE
	//////////////////////////////////

	data := map[string]interface{}{
		"Username": username,
		"Files":    files,
	}

	tmpl, err := template.ParseFiles("templates/base.html", "templates/list.html")
	if err != nil {
		log.Error(err)
	}
	err = tmpl.Execute(response, data)
	if err != nil {
		log.Error(err)
	}
}

func getFile(response http.ResponseWriter, request *http.Request, username string) {
	fileString := strings.TrimPrefix(request.URL.Path, "/file/")

	_ = fileString

	//////////////////////////////////
	// BEGIN TASK 5: YOUR CODE HERE
	//////////////////////////////////
	parts := strings.Split(fileString, "/")
	requestFile := parts[len(parts)-1]

	fileQuery := "SELECT * FROM files WHERE filename = ? AND (owner = ? OR shared LIKE " + "'%" + username + " %')"
	row := db.QueryRow(fileQuery, requestFile, username)

	var id int
	var owner, path, filename, shared string

	if fileErr := row.Scan(&id, &owner, &path, &filename, &shared); fileErr != nil {
		response.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(response, requestFile+": "+fileErr.Error()) //fileErr.Error()
		return
	}

	// Attach file and change filename
	http.ServeFile(response, request, path)
	setNameOfServedFile(response, filename)

	// replace this line
	//fmt.Fprintf(response, "placeholder")

	//////////////////////////////////
	// END TASK 5: YOUR CODE HERE
	//////////////////////////////////
}

func setNameOfServedFile(response http.ResponseWriter, fileName string) {
	response.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", fileName))
}

func processShare(response http.ResponseWriter, request *http.Request, sender string) {
	recipient := request.FormValue("username")
	filename := request.FormValue("filename")
	_ = filename

	if sender == recipient {
		response.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(response, "can't share with yourself")
		return
	}

	//////////////////////////////////
	// BEGIN TASK 6: YOUR CODE HERE
	//////////////////////////////////

	// Confirm recipient exists
	query := "SELECT username from users WHERE username = ?"
	row := db.QueryRow(query, recipient)
	var username string

	err := row.Scan(&username)
	if err == sql.ErrNoRows {
		response.WriteHeader(http.StatusNotFound)
		fmt.Fprintf(response, "unknown recipient")
		return
	} else if err != nil {
		response.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(response, err.Error())
		return
	}

	// Confirm file exists
	query = "SELECT * from files WHERE owner = ? AND filename = ?"
	row = db.QueryRow(query, sender, filename)
	var (
		id     int
		owner  string
		path   string
		fname  string
		shared string
	)

	err = row.Scan(&id, &owner, &path, &fname, &shared)
	if err == sql.ErrNoRows {
		response.WriteHeader(http.StatusNotFound)
		fmt.Fprintf(response, "unknown filename")
		return
	} else if err != nil {
		response.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(response, err.Error())
		return
	}

	// Add recipient to shared field
	shared += recipient + " "
	db.Exec("UPDATE files SET shared = ? WHERE id = ?", shared, id)

	//////////////////////////////////
	// END TASK 6: YOUR CODE HERE
	//////////////////////////////////

}

// Initiate a new session for the given username
func initSession(response http.ResponseWriter, username string) {
	// Generate session token
	sessionToken, err := randomByteString(16)
	if err != nil {
		response.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(response, err.Error())
		return
	}

	expires := time.Now().Add(sessionDuration)

	// Store session in database
	_, err = db.Exec("INSERT INTO sessions VALUES (NULL, ?, ?, ?)", username, sessionToken, expires.Unix())
	if err != nil {
		response.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(response, err.Error())
		return
	}

	// Set cookie with session data
	http.SetCookie(response, &http.Cookie{
		Name:     "session_token",
		Value:    sessionToken,
		Expires:  expires,
		SameSite: http.SameSiteStrictMode,
	})
}
