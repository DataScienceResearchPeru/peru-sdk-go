// This example uses synthetic data and makes no network calls.
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/DataScienceResearchPeru/peru-sdk-go/reniec"
	"github.com/DataScienceResearchPeru/peru-sdk-go/sunat"
)

func main() {
	service, err := reniec.NewMockIdentityService([]reniec.Identity{
		{DNI: "00000001", GivenNames: "PERSONA", FirstSurname: "PRUEBA", SecondSurname: "SINTETICA"},
	})
	if err != nil {
		log.Fatal(err)
	}
	person, err := service.Lookup(context.Background(), reniec.IdentityQuery{DNI: "00000001"})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("Identidad sintética:", person.GivenNames)
	result, err := sunat.ParseValidityResponse([]byte(`{"success":true,"data":{"estadoCp":1,"estadoRuc":"00","condDomiRuc":"00"}}`))
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("Estado de comprobante simulado:", result.DocumentStatus)
}
