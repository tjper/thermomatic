package server

import (
	"encoding/json"
	"net/http"
	"regexp"
	"strconv"

	"github.com/tjper/thermomatic/internal/client"
)

const (
	pathHealth   = "/health"
	pathReadings = "/readings/"
	pathStatus   = "/status/"
)

func (srv *Server) router() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc(pathHealth, srv.handleHealth())
	mux.HandleFunc(pathReadings, srv.handleReadings())
	mux.HandleFunc(pathStatus, srv.handleStatus())
	return mux
}

// handleHealth is an HTTP endpoint at path /health
//
// GET:
// Retrieve the health of the http server. 200 on healthy.
func (srv *Server) handleHealth() http.HandlerFunc {
	pathRE := regexp.MustCompile(`^(/health){1}$`)

	return func(w http.ResponseWriter, r *http.Request) {
		parts := pathRE.FindStringSubmatch(r.URL.RequestURI())
		if len(parts) != 2 {
			http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
			return
		}

		switch r.Method {
		case http.MethodGet:
			w.WriteHeader(http.StatusOK)
			return

		default:
			http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
			return
		}

	}
}

// handleReadings is an HTTP endpoint at path /readings/:imei.
//
// GET:
// Retrieve the most recent reading for specified IMEI. Endpoint responds with
// 200 and the most recent reading on success. If the IMEI is offline, the
// endpoint responds with a 205.
func (srv *Server) handleReadings() http.HandlerFunc {
	pathRE := regexp.MustCompile(`^(/readings/){1}(\d{15}){1}$`)
	type Response struct {
		Reading client.Reading
	}

	return func(w http.ResponseWriter, r *http.Request) {
		parts := pathRE.FindStringSubmatch(r.URL.RequestURI())
		if len(parts) != 3 {
			http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
			return
		}
		imei, err := strconv.Atoi(parts[2])
		if err != nil {
			http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
			return
		}

		switch r.Method {
		case http.MethodGet:
			c, ok := srv.clientMap.Load(uint64(imei))
			if !ok {
				http.Error(w, http.StatusText(http.StatusNoContent), http.StatusNoContent)
				return
			}
			srv.logInfo.Println(c)

			w.Header().Set("Content-Type", "application/json")
			response := Response{
				Reading: c.LastReading(),
			}
			srv.logInfo.Println(response)
			if err := json.NewEncoder(w).Encode(response); err != nil {
				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			}
			return

		default:
			http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
			return
		}

	}
}

// handleStatus is an HTTP endpoint at path /status/:imei.
//
// GET:
// If the imei is online the response status code is 200. If the imei is
// offline the response status code is 204.
func (srv *Server) handleStatus() http.HandlerFunc {
	pathRE := regexp.MustCompile(`^(/status/){1}(\d{15}){1}$`)

	return func(w http.ResponseWriter, r *http.Request) {
		parts := pathRE.FindStringSubmatch(r.URL.RequestURI())
		if len(parts) != 3 {
			http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
			return
		}
		imei, err := strconv.Atoi(parts[2])
		if err != nil {
			http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
			return
		}

		switch r.Method {
		case http.MethodGet:
			if !srv.clientMap.Exists(uint64(imei)) {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			w.WriteHeader(http.StatusOK)
			return

		default:
			http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
			return
		}
	}
}
