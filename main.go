package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	// "reflect"
	"strings"

	// "reflect"
	"net/url"

	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"github.com/michelooliveira/vinyl-store/database"
	"github.com/michelooliveira/vinyl-store/utils"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	// "encoding/json"
	"net/http"
)

var collection *mongo.Collection

var ctx = context.TODO()

var validate *validator.Validate

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

var fieldsAndMessages map[string]string

func getFilters(queryString url.Values) bson.M {
	var filters = bson.M{}
	if len(queryString["price"]) > 0 {
		filters["price"] = bson.M{
			"$regex": primitive.Regex{Pattern: fmt.Sprintf(".*%s.*", queryString["price"][0]), Options: "i"},
		}
	}
	if len(queryString["artist"]) > 0 {
		filters["artist"] = bson.M{
			"$regex": primitive.Regex{Pattern: fmt.Sprintf(".*%s.*", queryString["artist"][0]), Options: "i"},
		}
	}
	if len(queryString["title"]) > 0 {
		filters["title"] = bson.M{
			"$regex": primitive.Regex{Pattern: fmt.Sprintf(".*%s.*", queryString["title"][0]), Options: "i"},
		}
	}
	return filters
}

func getAlbums(c *gin.Context) {
	queryString := c.Request.URL.Query()
	var results []bson.M
	filter := getFilters(queryString)
	var page int64 = 1
	var perPage int64 = 10
	pageFromQuery := queryString["page"]
	perPageFromQuery := queryString["perPage"]
	if len(pageFromQuery) > 0 {
		convertedPageFromString, err := strconv.Atoi(pageFromQuery[0])
		if err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
				"message": "O parâmetro 'page' deve ser um número.",
			})
		}
		page = int64(convertedPageFromString)
	}
	if len(perPageFromQuery) > 0 {
		convertedPerPageFromString, err := strconv.Atoi(perPageFromQuery[0])
		if err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
				"message": "O parâmetro 'page' deve ser um número.",
			})
		}
		perPage = int64(convertedPerPageFromString)
	}

	findOptions := options.Find()
	findOptions.SetLimit(int64(perPage)) // SetLimit só aceita int64
	findOptions.SetSkip(int64(page-1) * int64(perPage))
	total, _ := collection.CountDocuments(ctx, filter)
	cursor, err := collection.Find(ctx, filter, findOptions)
	if err != nil {
		panic(err)
	}
	cursor.All(ctx, &results)
	c.IndentedJSON(http.StatusOK, gin.H{
		"data":    results,
		"total":   total,
		"page":    page,
		"perPage": perPage,
	})

}

func postAlbums(c *gin.Context) {
	var newAlbum newAlbum
	if err := c.ShouldBindJSON(&newAlbum); err != nil {
		var ve validator.ValidationErrors
		var jsonErr *json.UnmarshalTypeError
		if errors.As(err, &ve) {
			out := make([]ErrorMsg, len(ve))
			for i, fe := range ve {
				out[i] = ErrorMsg{fe.Field(), utils.GetErrorMsg(fe)}
			}
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"errors": out})
			return
		}
		if errors.As(err, &jsonErr) {
			out := make([]ErrorMsg, 1)
			messageForThisField := fieldsAndMessages[jsonErr.Field]
			out[0] = ErrorMsg{
				Field:   strings.ToLower(jsonErr.Field),
				Message: messageForThisField,
			}
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"errors": out})
			return
		}
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
	if err == mongo.ErrNoDocuments {
		c.IndentedJSON(http.StatusNotFound, gin.H{"message": "album not found"})
		return
	}
	c.IndentedJSON(http.StatusOK, result)
}

func updateAlbum(c *gin.Context) {
	id := utils.ConvertStringToObjectId(c.Param("id"))
	var fieldsToUpdate album
	if err := c.ShouldBindJSON(&fieldsToUpdate); err != nil {
		var jsonErr *json.UnmarshalTypeError
		if errors.As(err, &jsonErr) {
			out := make([]ErrorMsg, 1)
			messageForThisField := fieldsAndMessages[jsonErr.Field]
			out[0] = ErrorMsg{
				Field:   strings.ToLower(jsonErr.Field),
				Message: messageForThisField,
			}
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"errors": out})
			return
		}
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"errors": "Erro inesperado."})
		return
	}
	update := bson.D{{"$set", bson.D{
		{"artist", fieldsToUpdate.Artist},
		{"title", fieldsToUpdate.Title},
		{"price", fieldsToUpdate.Price},
	}}}
	// filter := bson.D{{"_id", id}}
	res, err := collection.UpdateByID(ctx, id, update)
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
	fieldsAndMessages = map[string]string{
		"title":  "O título deve ser do tipo string",
		"artist": "O artista deve ser do tipo string",
		"price":  "O preço deve ser do tipo float64",
	}
	validate = validator.New()
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
