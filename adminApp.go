package main

import (
	"github.com/hanahmily/iw-admin/service"
	"github.com/hanahmily/iw-admin/util"
	"log"
	"net/http"

	"github.com/ant0ine/go-json-rest/rest"
)

func handle_auth(w rest.ResponseWriter, r *rest.Request) {
	w.WriteJson(map[string]string{"authed": r.Env["REMOTE_USER"].(string)})
}

func main() {
	api := rest.NewApi()
	// var prodStack = []rest.Middleware{
	// 	&rest.TimerMiddleware{},
	// 	&rest.RecorderMiddleware{},
	// 	&rest.PoweredByMiddleware{},
	// 	&rest.RecoverMiddleware{},
	// 	&rest.GzipMiddleware{},
	// 	&rest.ContentTypeCheckerMiddleware{},
	// }
	api.Use(rest.DefaultDevStack...)
	// we use the IfMiddleware to remove certain paths from needing authentication
	jwtMiddleWare := service.AuthMiddleware()
	api.Use(&rest.IfMiddleware{
		Condition: func(request *rest.Request) bool {
			// return request.URL.Path == "/login"
			return false
		},
		IfTrue: &jwtMiddleWare,
	})
	api_router, _ := rest.MakeRouter(
		rest.Post("/login", jwtMiddleWare.LoginHandler),
		rest.Post("/spu/:oper", service.SpuHandle),
		rest.Post("/order/:oper", service.OrderHandle),
		rest.Post("/user/:oper", service.UserHandle),
		rest.Post("/topImage/:oper", service.TopImageHandle),
		rest.Get("/refresh_token", jwtMiddleWare.RefreshHandler),
	)
	api.SetApp(api_router)

	http.Handle("/api/", http.StripPrefix("/api", api.MakeHandler()))
	// http.HandleFunc("/upload", upload)
	http.HandleFunc("/upload", util.UploadHandle)

	log.Fatal(http.ListenAndServe(":9080", nil))
}
