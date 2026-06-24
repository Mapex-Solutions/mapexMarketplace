package handlers

import (
	"strconv"

	response "github.com/Mapex-Solutions/mapexGoKit/microservices/http/response"
	web "github.com/Mapex-Solutions/mapexGoKit/microservices/http/web"

	"mapexmarketplace/src/modules/devices/application/dtos"
	"mapexmarketplace/src/modules/devices/application/ports"
)

// contentTypeHeader is the response header GetAsset sets from the resolved type.
const contentTypeHeader = "Content-Type"

// ListDevices returns a handler that lists catalog devices, reading the filters
// and pagination from the query string.
//
// Returns 200 OK with the paginated CatalogListResponse.
func ListDevices(service ports.DevicesServicePort) web.Handler {
	return func(c *web.Ctx) error {
		query := &dtos.CatalogQuery{
			Protocol:     c.Query("protocol"),
			ReadingType:  c.Query("readingType"),
			Manufacturer: c.Query("manufacturer"),
			Search:       c.Query("search"),
			Lang:         c.Query("lang"),
			Page:         atoi(c.Query("page")),
			PerPage:      atoi(c.Query("perPage")),
		}
		retData, err := service.List(c.UserContext(), query)
		if err != nil {
			return err
		}
		return response.Success(c, retData)
	}
}

// GetFacets returns a handler that serves the listing filter options.
//
// Returns 200 OK with the Facets DTO.
func GetFacets(service ports.DevicesServicePort) web.Handler {
	return func(c *web.Ctx) error {
		retData, err := service.Facets(c.UserContext())
		if err != nil {
			return err
		}
		return response.Success(c, retData)
	}
}

// ListModelCodecs returns a handler that serves the codecs available for a model.
//
// Returns 200 OK with the array of Codec DTOs, or 404 if the model is unknown.
func ListModelCodecs(service ports.DevicesServicePort) web.Handler {
	return func(c *web.Ctx) error {
		retData, err := service.Codecs(c.UserContext(), c.Params("vendor"), c.Params("slug"))
		if err != nil {
			return err
		}
		return response.Success(c, retData)
	}
}

// GetInformation returns a handler that serves a model's information sheet.
//
// Returns 200 OK with the raw information JSON, or 404 if the model is unknown.
func GetInformation(service ports.DevicesServicePort) web.Handler {
	return func(c *web.Ctx) error {
		retData, err := service.GetInformation(c.UserContext(), c.Params("vendor"), c.Params("slug"))
		if err != nil {
			return err
		}
		return response.Success(c, retData)
	}
}

// GetSimulator returns a handler that serves a model's install template.
//
// Returns 200 OK with the raw simulator JSON, or 404 if the model is unknown.
func GetSimulator(service ports.DevicesServicePort) web.Handler {
	return func(c *web.Ctx) error {
		retData, err := service.GetSimulator(c.UserContext(), c.Params("vendor"), c.Params("slug"))
		if err != nil {
			return err
		}
		return response.Success(c, retData)
	}
}

// GetAsset returns a handler that streams a bundle asset (codec, manual, image)
// with its content type. The asset path is the wildcard segment.
//
// Returns 200 OK with the raw file, or 404 if the asset is unknown.
func GetAsset(service ports.DevicesServicePort) web.Handler {
	return func(c *web.Ctx) error {
		data, contentType, err := service.GetAsset(c.UserContext(), c.Params("vendor"), c.Params("slug"), c.Params("*"))
		if err != nil {
			return err
		}
		c.Set(contentTypeHeader, contentType)
		c.Set("Cache-Control", "public, max-age=86400") // bundle assets rarely change
		return c.Send(data)
	}
}

// atoi parses a query integer, treating any malformed value as zero so the
// service applies its defaults.
func atoi(value string) int {
	n, err := strconv.Atoi(value)
	if err != nil {
		return 0
	}
	return n
}
