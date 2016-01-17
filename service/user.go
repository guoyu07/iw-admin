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
	if userName, ok := query["address.username"]; ok {
		query["address.username"] = &bson.RegEx{Pattern: userName.(string), Options: "i"}
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

func becomeAmb(session *mgo.Session, userInfo map[string]interface{}) (ambCode string, err error) {
	objectId, ok := userInfo["_id"].(string)
	if !ok {
		return "", NotFoundFieldError
	}
	codeLock.Lock()
	defer codeLock.Unlock()
	change := mgo.Change{
		Update:    bson.M{"$inc": bson.M{"n": 1}},
		ReturnNew: true,
		Upsert:    true,
	}
	ambCodeInfo := make(map[string]interface{})
	_, err = session.DB("basic").C("ambCodeInfo").Find(bson.M{"_id": "code"}).Apply(change, ambCodeInfo)
	if err != nil {
		return "", err
	}
	ambCodeStr := strconv.Itoa(ambCodeInfo["n"].(int))
	buffer := bytes.NewBufferString("A")
	for i := 0; i < 4-len(ambCodeStr); i++ {
		buffer.WriteString("0")
	}
	buffer.WriteString(ambCodeStr)
	ambCode = buffer.String()
	change = mgo.Change{
		Update: bson.M{"$set": bson.M{"ambCode": ambCode}},
		Upsert: true,
	}
	_, err = session.DB("basic").C("user").Find(bson.M{"_id": objectId}).Apply(change, ambCodeInfo)
	if err != nil {
		return "", err
	}
	return ambCode, nil
}

func findLevel(session *mgo.Session) (result []map[string]interface{}, err error) {
	err = session.DB("basic").C("level").Find(bson.M{}).All(&result)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func saveLevel(session *mgo.Session, levelInfo map[string]interface{}) error {
	levelCol := session.DB("basic").C("level")
	dataArray := levelInfo["list"].([]interface{})
	for _, level := range dataArray {
		l := level.(map[string]interface{})
		id := l["_id"]
		delete(l, "_id")
		err := levelCol.Update(bson.M{"_id": id}, l)
		if err != nil {
			return err
		}
	}

	return nil
}

func UserHandle(w rest.ResponseWriter, r *rest.Request) {
	userInfo := make(map[string]interface{})
	err := r.DecodeJsonPayload(&userInfo)
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
		itemList, interErr := findUser(session, userInfo)
		if interErr == nil {
			err = w.WriteJson(itemList)
		} else {
			err = interErr
		}
	case "becomeAmb":
		ambCode, err := becomeAmb(session, userInfo)

		if err == nil {
			result := bson.M{"ambCode": ambCode}
			fmt.Println(result)
			err = w.WriteJson(result)
		}
	case "findLevel":
		itemList, interErr := findLevel(session)
		if interErr == nil {
			err = w.WriteJson(itemList)
		} else {
			err = interErr
		}
	case "saveLevel":
		err = saveLevel(session, userInfo)
	default:
		rest.Error(w, "user Not Implemented Operation", http.StatusNotImplemented)
	}
	if err != nil {
		rest.Error(w, fmt.Sprintf("user operation error:%s", err.Error()), http.StatusInternalServerError)
	}
}
