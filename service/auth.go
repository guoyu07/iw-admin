package service

import (
	"github.com/StephanDollberg/go-json-rest-middleware-jwt"
	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
	"time"
)

func auth(userId string, password string) bool {
	session, err := mgo.Dial("mongodb.iwshoes.cn")
	if err != nil {
		panic(err)
	}
	defer session.Close()

	c := session.DB("basic").C("auth")
	number, _ := c.Find(bson.M{"userId": userId, "password": password}).Count()
	return number == 1
}

func AuthMiddleware() jwt.JWTMiddleware {

	return jwt.JWTMiddleware{
		Key:           []byte("secret key"),
		Realm:         "jwt auth",
		Timeout:       time.Hour,
		MaxRefresh:    time.Hour * 24,
		Authenticator: auth}
}
