package service

import (
	"fmt"
	"github.com/ant0ine/go-json-rest/rest"
	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
	"net/http"
)

func changeTopImage(session *mgo.Session, imageInfo map[string]interface{}) error {
	c := session.DB("basic").C("image_arch")
	err := c.UpdateId("image", bson.M{"$set": imageInfo, "$currentDate": bson.M{"lastModified": true}})
	return err
}

func getTopImage(session *mgo.Session) (result map[string]interface{}, err error) {
	err = session.DB("basic").C("image_arch").FindId("image").One(&result)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func TopImageHandle(w rest.ResponseWriter, r *rest.Request) {
	imageInfo := make(map[string]interface{})
	err := r.DecodeJsonPayload(&imageInfo)
	if err != nil {
		rest.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	session, err := mgo.Dial("mongodb.iwshoes.cn")
	if err != nil {
		panic(err)
	}
	defer session.Close()
	// Optional. Switch the session to a monotonic behavior.
	session.SetMode(mgo.Monotonic, true)

	switch r.PathParam("oper") {
	case "get":
		itemList, interErr := getTopImage(session)
		if interErr == nil {
			err = w.WriteJson(itemList)
		} else {
			err = interErr
		}
	case "change":
		err = changeTopImage(session, imageInfo)
	default:
		rest.Error(w, "top image Not Implemented Operation", http.StatusNotImplemented)
	}
	if err != nil {
		rest.Error(w, fmt.Sprintf("top image operation error:%s", err.Error()), http.StatusInternalServerError)
	}
}
