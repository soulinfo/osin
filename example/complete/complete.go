package main

// Open url in browser:
// http://localhost:14000/app

import (
	example ".."
	osin "../.."
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
)

func main() {
	sconfig := osin.NewServerConfig()
	sconfig.AllowedAuthorizeTypes = osin.AllowedAuthorizeType{osin.CODE, osin.TOKEN}
	sconfig.AllowedAccessTypes = osin.AllowedAccessType{osin.AUTHORIZATION_CODE,
		osin.REFRESH_TOKEN, osin.PASSWORD, osin.CLIENT_CREDENTIALS}
	server := osin.NewServer(sconfig, example.NewTestStorage())
	output := osin.NewResponseOutputJSON()

	// Authorization code endpoint
	http.HandleFunc("/authorize", func(w http.ResponseWriter, r *http.Request) {
		resp := server.NewResponse()
		if ar := server.HandleAuthorizeRequest(resp, r); ar != nil {
			if !HandleLoginPage(ar, w, r) {
				return
			}
			ar.UserData = struct{ Login string }{Login: "test"}
			ar.Authorized = true
			server.FinishAuthorizeRequest(resp, r, ar)
		}
		if resp.IsError && resp.InternalError != nil {
			fmt.Printf("ERROR: %s\n", resp.InternalError)
		}
		if !resp.IsError {
			resp.Output["custom_parameter"] = 187723
		}
		output.Output(resp, w, r)
	})

	// Access token endpoint
	http.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		resp := server.NewResponse()
		if ar := server.HandleAccessRequest(resp, r); ar != nil {
			switch ar.Type {
			case osin.AUTHORIZATION_CODE:
				ar.Authorized = true
			case osin.REFRESH_TOKEN:
				ar.Authorized = true
			case osin.PASSWORD:
				if ar.Username == "test" && ar.Password == "test" {
					ar.Authorized = true
				}
			case osin.CLIENT_CREDENTIALS:
				ar.Authorized = true
			}
			server.FinishAccessRequest(resp, r, ar)
		}
		if resp.IsError && resp.InternalError != nil {
			fmt.Printf("ERROR: %s\n", resp.InternalError)
		}
		if !resp.IsError {
			resp.Output["custom_parameter"] = 19923
		}
		output.Output(resp, w, r)
	})

	// Information endpoint
	http.HandleFunc("/info", func(w http.ResponseWriter, r *http.Request) {
		resp := server.NewResponse()
		if ir := server.HandleInfoRequest(resp, r); ir != nil {
			server.FinishInfoRequest(resp, r, ir)
		}
		output.Output(resp, w, r)
	})

	// Application home endpoint
	http.HandleFunc("/app", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("<html><body>"))

		w.Write([]byte(fmt.Sprintf("<a href=\"/authorize?response_type=code&client_id=1234&state=xyz&scope=everything&redirect_uri=%s\">Code</a><br/>", url.QueryEscape("http://localhost:14000/appauth/code"))))
		w.Write([]byte(fmt.Sprintf("<a href=\"/authorize?response_type=token&client_id=1234&state=xyz&scope=everything&redirect_uri=%s\">Implict</a><br/>", url.QueryEscape("http://localhost:14000/appauth/token"))))
		w.Write([]byte(fmt.Sprintf("<a href=\"/appauth/password\">Password</a><br/>")))
		w.Write([]byte(fmt.Sprintf("<a href=\"/appauth/client_credentials\">Client Credentials</a><br/>")))

		w.Write([]byte("</body></html>"))
	})

	// Application destination - CODE
	http.HandleFunc("/appauth/code", func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()

		code := r.Form.Get("code")

		w.Write([]byte("<html><body>"))
		w.Write([]byte("APP AUTH - CODE<br/>"))

		if code != "" {
			jr := make(map[string]interface{})

			// build access code url
			aurl := fmt.Sprintf("/token?grant_type=authorization_code&client_id=1234&state=xyz&redirect_uri=%s&code=%s",
				url.QueryEscape("http://localhost:14000/appauth/code"), url.QueryEscape(code))

			// if parse, download and parse json
			if r.Form.Get("doparse") == "1" {
				err := DownloadAccessToken(fmt.Sprintf("http://localhost:14000%s", aurl),
					&osin.BasicAuth{"1234", "aabbccdd"}, jr)
				if err != nil {
					w.Write([]byte(err.Error()))
					w.Write([]byte("<br/>"))
				}
			}

			// show json error
			if erd, ok := jr["error"]; ok {
				w.Write([]byte(fmt.Sprintf("ERROR: %s<br/>\n", erd)))
			}

			// show json access token
			if at, ok := jr["access_token"]; ok {
				w.Write([]byte(fmt.Sprintf("ACCESS TOKEN: %s<br/>\n", at)))
			}

			w.Write([]byte(fmt.Sprintf("FULL RESULT: %+v<br/>\n", jr)))

			// output links
			w.Write([]byte(fmt.Sprintf("<a href=\"%s\">Goto Token URL</a><br/>", aurl)))

			cururl := *r.URL
			curq := cururl.Query()
			curq.Add("doparse", "1")
			cururl.RawQuery = curq.Encode()
			w.Write([]byte(fmt.Sprintf("<a href=\"%s\">Download Token</a><br/>", cururl.String())))

			if rt, ok := jr["refresh_token"]; ok {
				rurl := fmt.Sprintf("/appauth/refresh?code=%s", rt)
				w.Write([]byte(fmt.Sprintf("<a href=\"%s\">Refresh Token</a><br/>", rurl)))
			}

			if at, ok := jr["access_token"]; ok {
				rurl := fmt.Sprintf("/appauth/info?code=%s", at)
				w.Write([]byte(fmt.Sprintf("<a href=\"%s\">Info</a><br/>", rurl)))
			}

		} else {
			w.Write([]byte("Nothing to do"))
		}

		w.Write([]byte("</body></html>"))
	})

	// Application destination - TOKEN
	http.HandleFunc("/appauth/token", func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()

		w.Write([]byte("<html><body>"))
		w.Write([]byte("APP AUTH - TOKEN<br/>"))

		w.Write([]byte("Response data in fragment - not acessible via server - Nothing to do"))

		w.Write([]byte("</body></html>"))
	})

	// Application destination - PASSWORD
	http.HandleFunc("/appauth/password", func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()

		w.Write([]byte("<html><body>"))
		w.Write([]byte("APP AUTH - PASSWORD<br/>"))

		jr := make(map[string]interface{})

		// build access code url
		aurl := fmt.Sprintf("/token?grant_type=password&scope=everything&username=%s&password=%s",
			"test", "test")

		// doownload token
		err := DownloadAccessToken(fmt.Sprintf("http://localhost:14000%s", aurl),
			&osin.BasicAuth{Username: "1234", Password: "aabbccdd"}, jr)
		if err != nil {
			w.Write([]byte(err.Error()))
			w.Write([]byte("<br/>"))
		}

		// show json error
		if erd, ok := jr["error"]; ok {
			w.Write([]byte(fmt.Sprintf("ERROR: %s<br/>\n", erd)))
		}

		// show json access token
		if at, ok := jr["access_token"]; ok {
			w.Write([]byte(fmt.Sprintf("ACCESS TOKEN: %s<br/>\n", at)))
		}

		w.Write([]byte(fmt.Sprintf("FULL RESULT: %+v<br/>\n", jr)))

		if rt, ok := jr["refresh_token"]; ok {
			rurl := fmt.Sprintf("/appauth/refresh?code=%s", rt)
			w.Write([]byte(fmt.Sprintf("<a href=\"%s\">Refresh Token</a><br/>", rurl)))
		}

		if at, ok := jr["access_token"]; ok {
			rurl := fmt.Sprintf("/appauth/info?code=%s", at)
			w.Write([]byte(fmt.Sprintf("<a href=\"%s\">Info</a><br/>", rurl)))
		}

		w.Write([]byte("</body></html>"))
	})

	// Application destination - CLIENT_CREDENTIALS
	http.HandleFunc("/appauth/client_credentials", func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()

		w.Write([]byte("<html><body>"))
		w.Write([]byte("APP AUTH - CLIENT CREDENTIALS<br/>"))

		jr := make(map[string]interface{})

		// build access code url
		aurl := fmt.Sprintf("/token?grant_type=client_credentials")

		// doownload token
		err := DownloadAccessToken(fmt.Sprintf("http://localhost:14000%s", aurl),
			&osin.BasicAuth{Username: "1234", Password: "aabbccdd"}, jr)
		if err != nil {
			w.Write([]byte(err.Error()))
			w.Write([]byte("<br/>"))
		}

		// show json error
		if erd, ok := jr["error"]; ok {
			w.Write([]byte(fmt.Sprintf("ERROR: %s<br/>\n", erd)))
		}

		// show json access token
		if at, ok := jr["access_token"]; ok {
			w.Write([]byte(fmt.Sprintf("ACCESS TOKEN: %s<br/>\n", at)))
		}

		w.Write([]byte(fmt.Sprintf("FULL RESULT: %+v<br/>\n", jr)))

		if rt, ok := jr["refresh_token"]; ok {
			rurl := fmt.Sprintf("/appauth/refresh?code=%s", rt)
			w.Write([]byte(fmt.Sprintf("<a href=\"%s\">Refresh Token</a><br/>", rurl)))
		}

		if at, ok := jr["access_token"]; ok {
			rurl := fmt.Sprintf("/appauth/info?code=%s", at)
			w.Write([]byte(fmt.Sprintf("<a href=\"%s\">Info</a><br/>", rurl)))
		}

		w.Write([]byte("</body></html>"))
	})

	// Application destination - REFRESH
	http.HandleFunc("/appauth/refresh", func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()

		w.Write([]byte("<html><body>"))
		w.Write([]byte("APP AUTH - REFRESH<br/>"))

		code := r.Form.Get("code")

		if code != "" {
			jr := make(map[string]interface{})

			// build access code url
			aurl := fmt.Sprintf("/token?grant_type=refresh_token&refresh_token=%s", url.QueryEscape(code))

			// doownload token
			err := DownloadAccessToken(fmt.Sprintf("http://localhost:14000%s", aurl),
				&osin.BasicAuth{Username: "1234", Password: "aabbccdd"}, jr)
			if err != nil {
				w.Write([]byte(err.Error()))
				w.Write([]byte("<br/>"))
			}

			// show json error
			if erd, ok := jr["error"]; ok {
				w.Write([]byte(fmt.Sprintf("ERROR: %s<br/>\n", erd)))
			}

			// show json access token
			if at, ok := jr["access_token"]; ok {
				w.Write([]byte(fmt.Sprintf("ACCESS TOKEN: %s<br/>\n", at)))
			}

			w.Write([]byte(fmt.Sprintf("FULL RESULT: %+v<br/>\n", jr)))

			if rt, ok := jr["refresh_token"]; ok {
				rurl := fmt.Sprintf("/appauth/refresh?code=%s", rt)
				w.Write([]byte(fmt.Sprintf("<a href=\"%s\">Refresh Token</a><br/>", rurl)))
			}

			if at, ok := jr["access_token"]; ok {
				rurl := fmt.Sprintf("/appauth/info?code=%s", at)
				w.Write([]byte(fmt.Sprintf("<a href=\"%s\">Info</a><br/>", rurl)))
			}
		} else {
			w.Write([]byte("Nothing to do"))
		}

		w.Write([]byte("</body></html>"))
	})

	// Application destination - INFO
	http.HandleFunc("/appauth/info", func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()

		w.Write([]byte("<html><body>"))
		w.Write([]byte("APP AUTH - INFO<br/>"))

		code := r.Form.Get("code")

		if code != "" {
			jr := make(map[string]interface{})

			// build access code url
			aurl := fmt.Sprintf("/info?code=%s", url.QueryEscape(code))

			// doownload token
			err := DownloadAccessToken(fmt.Sprintf("http://localhost:14000%s", aurl),
				&osin.BasicAuth{Username: "1234", Password: "aabbccdd"}, jr)
			if err != nil {
				w.Write([]byte(err.Error()))
				w.Write([]byte("<br/>"))
			}

			// show json error
			if erd, ok := jr["error"]; ok {
				w.Write([]byte(fmt.Sprintf("ERROR: %s<br/>\n", erd)))
			}

			// show json access token
			if at, ok := jr["access_token"]; ok {
				w.Write([]byte(fmt.Sprintf("ACCESS TOKEN: %s<br/>\n", at)))
			}

			w.Write([]byte(fmt.Sprintf("FULL RESULT: %+v<br/>\n", jr)))

			if rt, ok := jr["refresh_token"]; ok {
				rurl := fmt.Sprintf("/appauth/refresh?code=%s", rt)
				w.Write([]byte(fmt.Sprintf("<a href=\"%s\">Refresh Token</a><br/>", rurl)))
			}
		} else {
			w.Write([]byte("Nothing to do"))
		}

		w.Write([]byte("</body></html>"))
	})

	http.ListenAndServe(":14000", nil)
}

func DownloadAccessToken(url string, auth *osin.BasicAuth, output map[string]interface{}) error {
	// download access token
	preq, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}
	if auth != nil {
		preq.SetBasicAuth(auth.Username, auth.Password)
	}

	pclient := &http.Client{}
	presp, err := pclient.Do(preq)
	if err != nil {
		return err
	}

	if presp.StatusCode != 200 {
		return errors.New("Invalid status code")
	}

	jdec := json.NewDecoder(presp.Body)
	err = jdec.Decode(&output)
	return err
}

// Login page
func HandleLoginPage(ar *osin.AuthorizeRequest, w http.ResponseWriter, r *http.Request) bool {
	r.ParseForm()
	if r.Method == "POST" && r.Form.Get("login") == "test" && r.Form.Get("password") == "test" {
		return true
	}

	w.Write([]byte("<html><body>"))

	w.Write([]byte(fmt.Sprintf("LOGIN %s (use test/test)<br/>", ar.Client.Id)))
	w.Write([]byte(fmt.Sprintf("<form action=\"/authorize?response_type=%s&client_id=%s&state=%s&redirect_uri=%s\" method=\"POST\">",
		ar.Type, ar.Client.Id, ar.State, url.QueryEscape(ar.RedirectUri))))

	w.Write([]byte("Login: <input type=\"text\" name=\"login\" /><br/>"))
	w.Write([]byte("Password: <input type=\"password\" name=\"password\" /><br/>"))
	w.Write([]byte("<input type=\"submit\"/>"))

	w.Write([]byte("</form>"))

	w.Write([]byte("</body></html>"))

	return false
}
