package service

import (
	"fmt"
	"github.com/ant0ine/go-json-rest/rest"
	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
	"net/http"
)

func updateOrder(session *mgo.Session, orderInfo map[string]interface{}) error {
	objectId := orderInfo["_id"]
	delete(orderInfo, "_id")
	c := session.DB("order").C("items")
	selector := bson.M{"_id": objectId}
	err := c.Update(selector, bson.M{"$set": orderInfo, "$currentDate": bson.M{"lastModified": true}})
	return err
}

func findOrder(session *mgo.Session, scenior map[string]interface{}) (result []map[string]interface{}, err error) {
	c := session.DB("order").C("items")

	query, ok := scenior["query"].(map[string]interface{})
	if !ok {
		return nil, NotFoundFieldError
	}
	sort, ok := scenior["sort"].(string)
	if !ok {
		return nil, NotFoundFieldError
	}

	err = c.Find(query).Select(bson.M{"msg": 0}).Sort(sort).All(&result)
	if err != nil {
		return nil, err
	}

	for _, item := range result {
		if _, ok := item["item"].(map[string]interface{}); !ok {
			continue
		}
		userInfo, err := findUserOne(session, map[string]interface{}{
			"_id": item["userId"],
		})
		if err != nil {
			continue
		}
		item["userInfo"] = userInfo

	}
	return result, err
}

func OrderHandle(w rest.ResponseWriter, r *rest.Request) {
	orderInfo := make(map[string]interface{})
	err := r.DecodeJsonPayload(&orderInfo)
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
	case "find":
		itemList, interErr := findOrder(session, orderInfo)
		if interErr == nil {
			err = w.WriteJson(itemList)
		} else {
			err = interErr
		}
	case "update":
		err = updateOrder(session, orderInfo)
	default:
		rest.Error(w, "order Not Implemented Operation", http.StatusNotImplemented)
	}
	if err != nil {
		rest.Error(w, fmt.Sprintf("order operation error:%s", err.Error()), http.StatusInternalServerError)
	}
}
