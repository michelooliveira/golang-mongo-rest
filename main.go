package main

import (
	"context"
	"errors"
	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"github.com/michelooliveira/vinyl-store/database"
	"github.com/michelooliveira/vinyl-store/utils"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"

	// "encoding/json"
	"net/http"
)

var collection *mongo.Collection

var ctx = context.TODO()

// album represents data about a record album.
type album struct {
	ID     string  `json:"id" bson:"_id, omitempty"`
	Title  string  `json:"title" bson:"title, omitempty"`
	Artist string  `json:"artist" bson:"artist, omitempty"`
	Price  float64 `json:"price" bson:"price, omitempty"`
}
type newAlbum struct {
	Title  string  `json:"title" bson:"title, omitempty" binding:"required"`
	Artist string  `json:"artist" bson:"artist, omitempty" binding:"required"`
	Price  float64 `json:"price" bson:"price, omitempty" binding:"required"`
}

type ErrorMsg struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

func getAlbums(c *gin.Context) {
	database.Connect()

	var results []bson.M
	cursor, err := collection.Find(ctx, bson.D{})
	if err != nil {
		panic(err)
	}
	cursor.All(ctx, &results)
	c.IndentedJSON(http.StatusOK, results)

}

func postAlbums(c *gin.Context) {
	var newAlbum newAlbum
	if err := c.ShouldBindJSON(&newAlbum); err != nil {
		var ve validator.ValidationErrors
		if errors.As(err, &ve) {
			out := make([]ErrorMsg, len(ve))
			for i, fe := range ve {
				out[i] = ErrorMsg{fe.Field(), utils.GetErrorMsg(fe)}
			}
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"errors": out})

		}
		return
	}
	res, err := collection.InsertOne(
		context.Background(),
		bson.M{"artist": newAlbum.Artist, "title": newAlbum.Title, "price": newAlbum.Price},
	)
	if err != nil {
		c.IndentedJSON(http.StatusBadRequest, gin.H{"erro": err})
		return
	}

	c.IndentedJSON(http.StatusCreated, res)
}

func getAlbumByID(c *gin.Context) {
	id := utils.ConvertStringToObjectId(c.Param("id"))
	var result bson.M
	err := collection.FindOne(ctx, bson.D{{"_id", id}}).Decode(&result)
	fmt.Print(err)
	if err == mongo.ErrNoDocuments {
		c.IndentedJSON(http.StatusNotFound, gin.H{"message": "album not found"})
		return
	}
	c.IndentedJSON(http.StatusOK, result)
}

func updateAlbum(c *gin.Context) {
	id := utils.ConvertStringToObjectId(c.Param("id"))
	var fieldsToUpdate album
	if err := c.BindJSON(&fieldsToUpdate); err != nil {
		panic(err)
	}
	// fmt.Print("Fields", fieldsToUpdate)
	update := bson.D{{"$set", bson.D{
		{"artist", fieldsToUpdate.Artist},
		{"title", fieldsToUpdate.Title},
		{"price", fieldsToUpdate.Price},
	}}}
	// filter := bson.D{{"_id", id}}
	res, err := collection.UpdateByID(ctx, id, update)
	fmt.Print(res)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			c.IndentedJSON(http.StatusNotFound, gin.H{"message": "album not found"})
			return
		} else {
			c.IndentedJSON(http.StatusBadRequest, gin.H{"erro": err})
		}
		panic(err)
	}

	c.IndentedJSON(http.StatusOK, gin.H{"message": res})
}

func deleteAlbum(c *gin.Context) {
	id := utils.ConvertStringToObjectId(c.Param("id"))
	res, err := collection.DeleteOne(ctx, bson.D{{"_id", id}})
	if err == mongo.ErrNoDocuments {
		c.IndentedJSON(http.StatusNotFound, gin.H{"message": "album not found"})
		return
	}
	c.IndentedJSON(http.StatusOK, gin.H{"message": "Album excluído com sucesso", "response": res})
}

func init() {
	database.Connect()
	collection = database.Collection
}

func main() {
	router := gin.Default()
	router.GET("/albums", getAlbums)
	router.POST("/albums", postAlbums)
	router.GET("/albums/:id", getAlbumByID)
	router.PATCH("/albums/:id", updateAlbum)
	router.DELETE("/albums/:id", deleteAlbum)
	router.Run(":8080")
}
