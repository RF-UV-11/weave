package ticket

import "servicesphere/backend-services/gen/backend_services/data_access/v1/dataaccessv1connect"

type Server struct {
	dataaccessv1connect.UnimplementedTicketServiceHandler
}
