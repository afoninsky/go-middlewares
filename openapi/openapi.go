package openapi

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
)

const specPath = "/spec.json"
const docPath = "/"

const docTemplate = `
<!DOCTYPE html>
<html>

<body>
    <redoc spec-url="./spec.json"></redoc>
    <script src="https://cdn.jsdelivr.net/npm/redoc@next/bundles/redoc.standalone.js"> </script>
</body>

</html>
`

// OpenAPI ...
type OpenAPI struct {
	spec   *openapi3.Swagger
	router *openapi3filter.Router
}

// NewFromFile creates openapi validation middleware
func NewFromFile(path string) (*OpenAPI, error) {
	spec, err := openapi3.NewSwaggerLoader().LoadSwaggerFromFile(path)
	if err != nil {
		return nil, err
	}
	return newFromSpec(spec)
}

// NewFromData creates openapi validation middleware
func NewFromData(data []byte) (*OpenAPI, error) {
	spec, err := openapi3.NewSwaggerLoader().LoadSwaggerFromData(data)
	if err != nil {
		return nil, err
	}
	return newFromSpec(spec)
}

func newFromSpec(spec *openapi3.Swagger) (*OpenAPI, error) {
	api := &OpenAPI{}
	api.router = openapi3filter.NewRouter()
	api.spec = spec
	if err := api.router.AddSwagger(spec); err != nil {
		return nil, err
	}
	return api, nil
}

// Middleware ...
// - ensures that requested route exists in openapi spec
// - validates passed parameters
func (api *OpenAPI) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		// serve static spec request
		if r.URL.Path == specPath && r.Method == http.MethodGet {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(api.spec)
			return
		}

		// serve documentation request
		if r.URL.Path == docPath && r.Method == http.MethodGet {
			io.WriteString(w, docTemplate)
			return
		}

		// workaround: https://github.com/getkin/kin-openapi/issues/118
		url := r.URL
		if r.Header.Get("Host") == "" {
			url.Host = "localhost"
			url.Scheme = "http"
		}

		// test if route exists in spec
		route, params, err := api.router.FindRoute(r.Method, url)
		if err != nil {
			switch err.Error() {
			case "Does not match any server":
				http.Error(w, "Host does not exist", http.StatusNotFound)
			case "Path was not found":
				http.Error(w, "Path does not exist", http.StatusNotFound)
			case "Path doesn't support the HTTP method":
				http.Error(w, "Path doesn't support the HTTP method", http.StatusBadRequest)
			default:
				http.Error(w, err.Error(), http.StatusBadRequest)
			}
			return
		}

		// validate request input
		input := &openapi3filter.RequestValidationInput{
			Request:    r,
			PathParams: params,
			Route:      route,
		}
		if err := openapi3filter.ValidateRequest(r.Context(), input); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		next.ServeHTTP(w, r)

		// TODO: optional: validate response output
		// output := &openapi3filter.ResponseValidationInput{
		// 	RequestValidationInput: input,
		// 	// Status:                 r.Response.StatusCode,
		// 	// Header: http.Header{
		// 	// 	"Content-Type": []string{respContentType},
		// 	// },
		// }
		// // if respBody != nil {
		// // 	data, _ := json.Marshal(respBody)
		// // 	responseValidationInput.SetBodyBytes(data)
		// // }

		// if err := openapi3filter.ValidateResponse(r.Context(), output); err != nil {
		// 	http.Error(w, err.Error(), 400)
		// 	return
		// }
	})
}
