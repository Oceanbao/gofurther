package main

import (
	"net/http"
)

// HealthCheck godoc
//
//	@Summary		Check server health
//	@ID				health-check
//	@Description	Respond 200 for OK
//	@Tags			health
//	@Produce		json
//	@Success		200
//	@Router			/healthcheck [get]
func (app *application) healthcheckHandler(w http.ResponseWriter, r *http.Request) {
	// js := `{"status": "available", "environment": %q, "version": %q}`
	// js = fmt.Sprintf(js, app.config.env, version)
	env := envelope{
		"status": "available",
		"system_info": map[string]string{
			"environment": app.config.env,
			"version":     version,
		},
	}

	err := app.writeJSON(w, http.StatusOK, env, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}
