package master

import "fmt"

const swaggerUIHTML = `<!DOCTYPE html>
<html>
<head>
  <title>NodePass API</title>
  <link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/swagger-ui-dist@5/swagger-ui.css">
</head>
<body>
  <div id="swagger-ui"></div>
  <script src="https://cdn.jsdelivr.net/npm/swagger-ui-dist@5/swagger-ui-bundle.js"></script>
  <script>
	window.onload = () => SwaggerUIBundle({
	  spec: %s,
	  dom_id: '#swagger-ui',
	  presets: [SwaggerUIBundle.presets.apis, SwaggerUIBundle.SwaggerUIStandalonePreset],
	  layout: "BaseLayout"
	});
  </script>
</body>
</html>`

func (m *Master) generateOpenAPISpec() string {
	return fmt.Sprintf(`{
  "openapi": "3.1.1",
  "info": {
	"title": "NodePass API",
	"description": "API for managing NodePass server and client instances",
	"version": "%s"
  },
  "servers": [{"url": "%s"}],
  "security": [{"ApiKeyAuth": []}],
  "paths": {
	"/instances": {
	  "get": {
		"summary": "List all instances",
		"security": [{"ApiKeyAuth": []}],
		"responses": {
		  "200": {"description": "Success", "content": {"application/json": {"schema": {"type": "array", "items": {"$ref": "#/components/schemas/Instance"}}}}},
		  "401": {"description": "Unauthorized"},
		  "405": {"description": "Method not allowed"}
		}
	  },
	  "post": {
		"summary": "Create a new instance",
		"security": [{"ApiKeyAuth": []}],
		"requestBody": {"required": true, "content": {"application/json": {"schema": {"$ref": "#/components/schemas/CreateInstanceRequest"}}}},
		"responses": {
		  "201": {"description": "Created", "content": {"application/json": {"schema": {"$ref": "#/components/schemas/Instance"}}}},
		  "400": {"description": "Invalid input"},
		  "401": {"description": "Unauthorized"},
		  "405": {"description": "Method not allowed"},
		  "409": {"description": "Instance ID already exists"}
		}
	  }
	},
	"/instances/{id}": {
	  "parameters": [{"name": "id", "in": "path", "required": true, "schema": {"type": "string"}}],
	  "get": {
		"summary": "Get instance details",
		"security": [{"ApiKeyAuth": []}],
		"responses": {
		  "200": {"description": "Success", "content": {"application/json": {"schema": {"$ref": "#/components/schemas/Instance"}}}},
		  "400": {"description": "Instance ID required"},
		  "401": {"description": "Unauthorized"},
		  "404": {"description": "Not found"},
		  "405": {"description": "Method not allowed"}
		}
	  },
	  "patch": {
		"summary": "Update instance",
		"security": [{"ApiKeyAuth": []}],
		"requestBody": {"required": true, "content": {"application/json": {"schema": {"$ref": "#/components/schemas/UpdateInstanceRequest"}}}},
		"responses": {
		  "200": {"description": "Success", "content": {"application/json": {"schema": {"$ref": "#/components/schemas/Instance"}}}},
		  "400": {"description": "Instance ID required or invalid input"},
		  "401": {"description": "Unauthorized"},
		  "404": {"description": "Not found"},
		  "405": {"description": "Method not allowed"}
		}
	  },
	  "put": {
		"summary": "Update instance URL",
		"security": [{"ApiKeyAuth": []}],
		"requestBody": {"required": true, "content": {"application/json": {"schema": {"$ref": "#/components/schemas/PutInstanceRequest"}}}},
		"responses": {
		  "200": {"description": "Success", "content": {"application/json": {"schema": {"$ref": "#/components/schemas/Instance"}}}},
		  "400": {"description": "Instance ID required or invalid input"},
		  "401": {"description": "Unauthorized"},
		  "403": {"description": "Forbidden"},
		  "404": {"description": "Not found"},
		  "405": {"description": "Method not allowed"},
		  "409": {"description": "Instance URL conflict"}
		}
	  },
	  "delete": {
		"summary": "Delete instance",
		"security": [{"ApiKeyAuth": []}],
		"responses": {
		  "204": {"description": "Deleted"},
		  "400": {"description": "Instance ID required"},
		  "401": {"description": "Unauthorized"},
		  "403": {"description": "Forbidden"},
		  "404": {"description": "Not found"},
		  "405": {"description": "Method not allowed"}
		}
	  }
	},
	"/events": {
	  "get": {
		"summary": "Subscribe to instance events",
		"security": [{"ApiKeyAuth": []}],
		"responses": {
		  "200": {"description": "Success", "content": {"text/event-stream": {}}},
		  "401": {"description": "Unauthorized"},
		  "405": {"description": "Method not allowed"}
		}
	  }
	},
	"/info": {
	  "get": {
		"summary": "Get master information",
		"security": [{"ApiKeyAuth": []}],
		"responses": {
		  "200": {"description": "Success", "content": {"application/json": {"schema": {"$ref": "#/components/schemas/MasterInfo"}}}},
		  "401": {"description": "Unauthorized"},
		  "405": {"description": "Method not allowed"}
		}
	  },
	  "post": {
		"summary": "Update master alias",
		"security": [{"ApiKeyAuth": []}],
		"requestBody": {"required": true, "content": {"application/json": {"schema": {"$ref": "#/components/schemas/UpdateMasterAliasRequest"}}}},
		"responses": {
		  "200": {"description": "Success", "content": {"application/json": {"schema": {"$ref": "#/components/schemas/MasterInfo"}}}},
		  "400": {"description": "Invalid input"},
		  "401": {"description": "Unauthorized"},
		  "405": {"description": "Method not allowed"}
		}
	  }
	},
	"/tcping": {
	  "get": {
		"summary": "TCP connectivity test",
		"security": [{"ApiKeyAuth": []}],
		"parameters": [
		  {
			"name": "target",
			"in": "query",
			"required": true,
			"schema": {"type": "string"},
			"description": "Target address in format host:port"
		  }
		],
		"responses": {
		  "200": {"description": "Success", "content": {"application/json": {"schema": {"$ref": "#/components/schemas/TCPingResult"}}}},
		  "400": {"description": "Target address required"},
		  "401": {"description": "Unauthorized"},
		  "405": {"description": "Method not allowed"}
		}
	  }
	},
	"/openapi.json": {
	  "get": {
		"summary": "Get OpenAPI specification",
		"responses": {
		  "200": {"description": "Success", "content": {"application/json": {}}}
		}
	  }
	},
	"/docs": {
	  "get": {
		"summary": "Get Swagger UI",
		"responses": {
		  "200": {"description": "Success", "content": {"text/html": {}}}
		}
	  }
	}
  },
  "components": {
   "securitySchemes": {
	 "ApiKeyAuth": {
	"type": "apiKey",
	"in": "header",
	"name": "X-API-Key",
	"description": "API Key for authentication"
	 }
   },
   "schemas": {
	 "Instance": {
	"type": "object",
	"properties": {
	  "id": {"type": "string", "description": "Unique identifier"},
	  "alias": {"type": "string", "description": "Instance alias"},
	  "type": {"type": "string", "enum": ["client", "server"], "description": "Type of instance"},
	  "status": {"type": "string", "enum": ["running", "stopped", "error"], "description": "Instance status"},
	  "url": {"type": "string", "description": "Command string or API Key"},
	  "config": {"type": "string", "description": "Instance configuration URL"},
	  "restart": {"type": "boolean", "description": "Restart policy"},
	  "meta": {"$ref": "#/components/schemas/Meta"},
	  "mode": {"type": "integer", "description": "Instance mode"},
	  "ping": {"type": "integer", "description": "TCPing latency"},
	  "pool": {"type": "integer", "description": "Pool active count"},
	  "tcps": {"type": "integer", "description": "TCP connection count"},
	  "udps": {"type": "integer", "description": "UDP connection count"},
	  "tcprx": {"type": "integer", "description": "TCP received bytes"},
	  "tcptx": {"type": "integer", "description": "TCP transmitted bytes"},
	  "udprx": {"type": "integer", "description": "UDP received bytes"},
	  "udptx": {"type": "integer", "description": "UDP transmitted bytes"}
	}
	 },
	  "CreateInstanceRequest": {
		"type": "object",
		"required": ["url"],
		"properties": {
		  "alias": {"type": "string", "description": "Instance alias"},
		  "url": {"type": "string", "description": "Command string(scheme://host:port/host:port)"}
		}
	  },
	  "UpdateInstanceRequest": {
		"type": "object",
		"properties": {
		  "alias": {"type": "string", "description": "Instance alias"},
		  "action": {"type": "string", "enum": ["start", "stop", "restart", "reset"], "description": "Action for the instance"},
		  "restart": {"type": "boolean", "description": "Instance restart policy"},
		  "meta": {"$ref": "#/components/schemas/Meta"}
		}
	  },
	  "PutInstanceRequest": {
		"type": "object",
		"required": ["url"],
		"properties": {"url": {"type": "string", "description": "New command string(scheme://host:port/host:port)"}}
	  },
	  "Meta": {
		"type": "object",
		"properties": {
		  "peer": {"$ref": "#/components/schemas/Peer"},
		  "tags": {"type": "object", "additionalProperties": {"type": "string"}, "description": "Key-value tags"}
		}
	  },
	  "Peer": {
		"type": "object",
		"properties": {
		  "sid": {"type": "string", "description": "Service ID"},
		  "type": {"type": "string", "description": "Service type"},
		  "alias": {"type": "string", "description": "Service alias"}
		}
	  },
	  "MasterInfo": {
		"type": "object",
		"properties": {
		  "mid": {"type": "string", "description": "Master ID"},
		  "alias": {"type": "string", "description": "Master alias"},
		  "os": {"type": "string", "description": "Operating system"},
		  "arch": {"type": "string", "description": "System architecture"},
		  "noc": {"type": "integer", "description": "Number of CPU cores"},
		  "cpu": {"type": "integer", "description": "CPU usage percentage"},
		  "mem_total": {"type": "integer", "format": "int64", "description": "Total memory in bytes"},
		  "mem_used": {"type": "integer", "format": "int64", "description": "Used memory in bytes"},
		  "swap_total": {"type": "integer", "format": "int64", "description": "Total swap space in bytes"},
		  "swap_used": {"type": "integer", "format": "int64", "description": "Used swap space in bytes"},
		  "netrx": {"type": "integer", "format": "int64", "description": "Network received bytes"},
		  "nettx": {"type": "integer", "format": "int64", "description": "Network transmitted bytes"},
		  "diskr": {"type": "integer", "format": "int64", "description": "Disk read bytes"},
		  "diskw": {"type": "integer", "format": "int64", "description": "Disk write bytes"},
		  "sysup": {"type": "integer", "format": "int64", "description": "System uptime in seconds"},
		  "ver": {"type": "string", "description": "NodePass version"},
		  "name": {"type": "string", "description": "Hostname"},
		  "uptime": {"type": "integer", "format": "int64", "description": "API uptime in seconds"},
		  "log": {"type": "string", "description": "Log level"},
		  "tls": {"type": "string", "description": "TLS code"},
		  "crt": {"type": "string", "description": "Certificate path"},
		  "key": {"type": "string", "description": "Private key path"}
		}
	  },
	  "UpdateMasterAliasRequest": {
		"type": "object",
		"required": ["alias"],
		"properties": {"alias": {"type": "string", "description": "Master alias"}}
	  },
	  "TCPingResult": {
		"type": "object",
		"properties": {
		  "target": {"type": "string", "description": "Target address"},
		  "connected": {"type": "boolean", "description": "Is connected"},
		  "latency": {"type": "integer", "format": "int64", "description": "Latency in milliseconds"},
		  "error": {"type": "string", "nullable": true, "description": "Error message"}
		}
	  }
	}
  }
}`, openAPIVersion, m.prefix)
}
