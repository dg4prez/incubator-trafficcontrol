package API::Metadata;

#
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
#
#
#
use UI::Utils;
use API::Configs::ApacheTrafficServer;
use Mojo::Base 'Mojolicious::Controller';
use Data::Dumper;
use UI::DeliveryService;
use JSON;

#Sub to generate config metadata
sub get_metadata {
	my $self     = shift;
	my $id       = $self->param('id');

	##check user access
	if ( !&is_oper($self) ) {
		return $self->forbidden();
	}

	##verify that a valid server ID has been used
	my $server_obj = $self->API::Configs::ApacheTrafficServer::server_data($id);
	if ( !defined($server_obj) ) {
		return $self->not_found();
	}

	my $data_obj;
	my $host_name = $server_obj->host_name;

	my %condition = ( 'me.host_name' => $host_name );
	my $tm_url = $self->db->resultset('Parameter')->search( { -and => [ name => 'tm.url', config_file => 'global' ] } )->get_column('value')->first();
	my $tm_rev_proxy_url = $self->db->resultset('Parameter')->search( { -and => [ name => 'tm.rev_proxy.url', config_file => 'global' ] } )->get_column('value')->first();
	if ( !$tm_rev_proxy_url ) {
	  $tm_rev_proxy_url = $tm_url;
	}
	my @ds_list =$self->get_ds_list($server_obj);

	if ($server_obj ) {
		$data_obj = {
			"hostName"	 => $server_obj->host_name,
		  "hostIpv4"	 => $server_obj->ip_address,
		  "hostId"		 => $server_obj->id,
		  "profileName"	 => $server_obj->profile->name,
		  "profileId"		 => $server_obj->profile->id,
		  "cdnName"		 => $server_obj->cdn->name,
		  "cdnId"			 => $server_obj->cdn->id,
		  "toUrl"			 => $tm_url,
		  "tcpPort" => $server_obj->tcp_port,
		  "status" => $server_obj->status->name,
		  "cachegroup" => $server_obj->cachegroup->name,
		  "type" => $server_obj->type->name,
			"toRevProxyUrl"	=> $tm_rev_proxy_url,
			"dsCount" => scalar(@ds_list),
			"dsList" => \@ds_list
		}
	}

my @response_data;
push @response_data, $data_obj;

return $self->success( @response_data );
}

sub get_ds_list{
	my $self = shift;
	my $server_obj = shift;

	my @ds_list;
	my $rs_dsinfo;
	my $ds_data;
	
	if ( $server_obj->type->name =~ m/^MID/ ) {
		# the mids will do all deliveryservices in this CDN
		$rs_dsinfo = $self->db->resultset('DeliveryServiceInfoForDomainList')->search( {}, { bind => [ $server_obj->cdn->name ] } );
	}
	else {
		$rs_dsinfo = $self->db->resultset('DeliveryServiceInfoForServerList')->search( {}, { bind => [ $server_obj->id ] } );
	}

	my $j = 0;
	while ( my $dsinfo = $rs_dsinfo->next ) {
		my $ds = $self->db->resultset('Deliveryservice')->search( { xml_id => $dsinfo->xml_id } )->single();
		$ds_data->{$j}->{'xmlID'} = $ds->xml_id;
		$ds_data->{$j}->{'version'} = $ds->last_updated;
		$ds_data->{$j}->{'id'} = $ds->id;
		push (@ds_list, $ds_data->{$j});

		$j++;
	}
	
	return @ds_list;
}

1;