package services


import (

"strconv"

"github.com/gofiber/fiber/v2"


"afrigoals.com/database"
"afrigoals.com/models"

)



func GenerateMatchClips(c *fiber.Ctx) error {



matchID,_ :=
strconv.Atoi(
c.Params("match_id"),
)



var req struct{

EventIDs []uint `json:"event_ids"`

}



if err:=c.BodyParser(&req);err!=nil{


return c.Status(400).JSON(
fiber.Map{
"error":"invalid request",
})

}




for _,eventID:=range req.EventIDs{


clip:=models.Clip{


MatchID:uint(matchID),


EventID:&eventID,


Title:"Match Event Clip",


Status:"pending",


StartSec:0,

}



database.DB.Create(&clip)



go ProcessClip(clip.ID)



}




return c.JSON(
fiber.Map{


"success":true,


"message":"Clip generation started",


"total":len(req.EventIDs),


})

}