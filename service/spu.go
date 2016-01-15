package service

import (
	"errors"
	"fmt"
	"github.com/ant0ine/go-json-rest/rest"
	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
	"net/http"
)

var (
	NotFoundFieldError = errors.New("can find specail field")
)

func addSpu(session *mgo.Session, spuInfo map[string]interface{}) error {
	c := session.DB("product").C("items")
	objectId := bson.NewObjectId()
	spuInfo["_id"] = objectId
	spuInfo["saleState"] = "on"
	spuInfo["onSaleDate"] = bson.Now()
	err := c.Insert(&spuInfo)
	return err
}

func updateSpu(session *mgo.Session, spuInfo map[string]interface{}) error {
	objectId := bson.ObjectIdHex(spuInfo["_id"].(string))
	delete(spuInfo, "_id")
	c := session.DB("product").C("items")
	selector := bson.M{"_id": objectId}
	err := c.Update(selector, bson.M{"$set": spuInfo, "$currentDate": bson.M{"lastModified": true}})
	return err
}

func toggleOnSale(session *mgo.Session, spuInfo map[string]interface{}) error {
	objectId, ok := spuInfo["_id"].(string)
	if !ok {
		return NotFoundFieldError
	}
	item, err := findSpuOne(session, spuInfo)
	if err != nil {
		return err
	}
	if item["saleState"] == "on" {
		//off sale
		return updateSpu(session, bson.M{"_id": objectId, "saleState": "off", "offSaleDate": bson.Now()})
	} else {
		//on sale
		return updateSpu(session, bson.M{"_id": objectId, "saleState": "on", "onSaleDate": bson.Now()})
	}
}

func findSpu(session *mgo.Session, scenior map[string]interface{}) (result []map[string]interface{}, err error) {
	c := session.DB("product").C("items")
	query, ok := scenior["query"].(map[string]interface{})
	if !ok {
		return nil, NotFoundFieldError
	}
	sort, ok := scenior["sort"].(string)
	if !ok {
		return nil, NotFoundFieldError
	}

	err = c.Find(query).Select(bson.M{"sizeList": 0, "itemStyle": 0, "desc": 0}).Sort(sort).All(&result)
	return result, err
}

func findSpuOne(session *mgo.Session, query map[string]interface{}) (result map[string]interface{}, err error) {
	objectId, ok := query["_id"].(string)
	if !ok {
		return nil, NotFoundFieldError
	}
	c := session.DB("product").C("items")
	err = c.FindId(bson.ObjectIdHex(objectId)).One(&result)
	return result, err
}

func SpuHandle(w rest.ResponseWriter, r *rest.Request) {
	spuInfo := make(map[string]interface{})
	err := r.DecodeJsonPayload(&spuInfo)
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
	case "add":
		err = addSpu(session, spuInfo)
	case "update":
		err = updateSpu(session, spuInfo)
	case "toggleOnSale":
		err = toggleOnSale(session, spuInfo)
	case "find":
		itemList, interErr := findSpu(session, spuInfo)
		if interErr == nil {
			err = w.WriteJson(itemList)
		} else {
			err = interErr
		}
	case "findOne":
		itemList, interErr := findSpuOne(session, spuInfo)
		if interErr == nil {
			err = w.WriteJson(itemList)
		} else {
			err = interErr
		}
	default:
		rest.Error(w, "spu Not Implemented Operation", http.StatusNotImplemented)
	}

	if err != nil {
		rest.Error(w, fmt.Sprintf("spu operation error:%s", err.Error()), http.StatusInternalServerError)
	}
}
