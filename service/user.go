package service

import (
	"bytes"
	"fmt"
	"github.com/ant0ine/go-json-rest/rest"
	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
	"net/http"
	"strconv"
	"sync"
)

var (
	codeLock sync.Mutex
)

func updateUser(session *mgo.Session, orderInfo map[string]interface{}) error {
	objectId := orderInfo["_id"]
	delete(orderInfo, "_id")
	c := session.DB("basic").C("user")
	selector := bson.M{"_id": objectId}
	err := c.Update(selector, bson.M{"$set": orderInfo, "$currentDate": bson.M{"lastModified": true}})
	return err
}

func findUser(session *mgo.Session, scenior map[string]interface{}) (result []map[string]interface{}, err error) {
	query, ok := scenior["query"].(map[string]interface{})
	if !ok {
		return nil, NotFoundFieldError
	}
	sort, ok := scenior["sort"].(string)
	if !ok {
		return nil, NotFoundFieldError
	}
	err = session.DB("basic").C("user").Find(query).Sort(sort).All(&result)
	if err != nil {
		return nil, err
	}

	for _, item := range result {
		appendAmbInfo(session, item)
	}
	return result, err
}

func findUserOne(session *mgo.Session, query map[string]interface{}) (result map[string]interface{}, err error) {
	objectId, ok := query["_id"].(string)
	if !ok {
		return nil, NotFoundFieldError
	}
	err = session.DB("basic").C("user").FindId(objectId).One(&result)
	if err != nil {
		return nil, err
	}
	appendAmbInfo(session, result)
	return result, nil
}

func appendAmbInfo(session *mgo.Session, userInfo map[string]interface{}) error {
	if packet, ok := userInfo["packet"].(map[string]interface{}); ok {
		ambInfo := make(map[string]interface{})
		err := session.DB("basic").C("user").FindId(packet["from"]).Select(bson.M{"_id": 0, "score": 0, "address": bson.M{"$slice": 1}}).One(ambInfo)
		if err != nil {
			return err
		}
		userInfo["ambInfo"] = ambInfo
	}
	return nil
}

func BecomeAmb(session *mgo.Session, userInfo map[string]interface{}) error {
	objectId, ok := userInfo["_id"].(string)
	if !ok {
		return NotFoundFieldError
	}
	codeLock.Lock()
	defer codeLock.Unlock()
	change := mgo.Change{
		Update:    bson.M{"$inc": bson.M{"n": 1}},
		ReturnNew: true,
		Upsert:    true,
	}
	ambCode := make(map[string]interface{})
	info, err := session.DB("basic").C("ambCode").Find(bson.M{"_id": "code"}).Apply(change, ambCode)
	if err != nil {
		return err
	}
	ambCodeStr := strconv.Itoa(ambCode["n"].(int))
	buffer := bytes.NewBufferString("A")
	for i := 0; i < 4-len(ambCodeStr); i++ {
		buffer.WriteString("0")
	}
	buffer.WriteString(ambCodeStr)
	change = mgo.Change{
		Update: bson.M{"$set": bson.M{"ambCode": buffer.String()}},
		Upsert: true,
	}
	info, err = session.DB("basic").C("user").Find(bson.M{"_id": "code"}).Apply(change, ambCode)
	if err != nil {
		return err
	}
	return nil
}

func UserHandle(w rest.ResponseWriter, r *rest.Request) {
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
		itemList, interErr := findUser(session, orderInfo)
		if interErr == nil {
			err = w.WriteJson(itemList)
		} else {
			err = interErr
		}
	case "update":
		err = updateUser(session, orderInfo)
	default:
		rest.Error(w, "order Not Implemented Operation", http.StatusNotImplemented)
	}
	if err != nil {
		rest.Error(w, fmt.Sprintf("order operation error:%s", err.Error()), http.StatusInternalServerError)
	}
}
