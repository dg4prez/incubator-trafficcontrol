package main

/*
 * Licensed to the Apache Software Foundation (ASF) under one
 * or more contributor license agreements.  See the NOTICE file
 * distributed with this work for additional information
 * regarding copyright ownership.  The ASF licenses this file
 * to you under the Apache License, Version 2.0 (the
 * "License"); you may not use this file except in compliance
 * with the License.  You may obtain a copy of the License at
 *
 *   http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

import (
	"database/sql"
	"time"
	//"encoding/json"
	"fmt"
	"net/http"
	"strings"

	//"github.com/lib/pq"

	"github.com/apache/incubator-trafficcontrol/traffic_monitor_golang/common/log"
)

// ServerObj ...
type ServerObj struct {
	DomainName    string
	HostName      string
	ID            int
	LastUpdated   string
	CachegroupID  int
	Cachegroup    string
	ProfileID     int
	Profile       string
	CDNID         int
	CDN           string
	Status        string
	TCPPort       int
	Type          string
	UpdatePending bool
	ATSVersion    string
}

func atsServerScopeConfigs(db *sql.DB) RegexHandlerFunc {
	return func(w http.ResponseWriter, r *http.Request, p ParamMap) {
		handleErr := func(err error, status int) {
			log.Errorf("%v %v\n", r.RemoteAddr, err)
			w.WriteHeader(status)
			fmt.Fprintf(w, http.StatusText(status))
		}

		hostName := p["host"]
		fileName := p["filename"]

		s, err := buildServerObj(db, hostName)
		if err != nil {
			log.Errorf("Error: %v\n", err)
			handleErr(err, http.StatusInternalServerError)
			return
		}

		header, err := headerComment(db, hostName)
		if err != nil {
			log.Errorf("Error: %v\n", err)
			handleErr(err, http.StatusInternalServerError)
			return
		}

		text := ""

		switch fileName {
		case "parent.config":
			text, err = parentConfig(db, s, header)
		}
		if err != nil {
			log.Errorf("Error: %v\n", err)
			handleErr(err, http.StatusInternalServerError)
			return
		}
		//log.Debugln(err)
		w.Header().Set("Content-Type", "text/plain")
		fmt.Fprintf(w, "%s", text)

	}
}

//need to merge into branch based on current master, validate parent_dot_config changes
func parentConfig(db *sql.DB, s ServerObj, header string) (string, error) {
	text := ""
	dsList, err := DeliveryServiceData(db, s)
	if err != nil {
		return text, err
	}

	if strings.HasPrefix(s.Type, "MID") {
		log.Debugln("mid found")

	} else {
		log.Debugln("not a mid")
	}
	text = header + s.ATSVersion + s.Type
	return text, nil
}

// DSList struct
type DSList struct {
	DeliveryServices []DSData
}

// DSData struct
type DSData struct {
	ID                    int
	XMLId                 string
	OrgServerFQDN         string
	Type                  string
	QStringIgnore         int
	OriginShield          string
	MultiSiteOrigin       bool
	Profile               int
	ParentQstringHandling string
	Params                []Parameters
}

// Parameters struct
type Parameters struct {
	ConfigFile string
	Name       string
	Value      string
}

func DeliveryServiceData(db *sql.DB, s ServerObj) (DSList, error) {
	dsList := DSList{}
	query := ""
	if strings.HasPrefix(s.Type, "MID") {
		query = `SELECT
		ds.id as id,
		ds.xml_id as xml_id,
		ds.org_server_fqdn as org_server_fqdn,
		type.name as type,
		ds.qstring_ignore as qstring_ignore,
		ds.origin_shield as origin_shield,
		ds.multi_site_origin as multi_site_origin,
		ds.profile as profile_id
		FROM deliveryservice ds
		JOIN type type on type.id = ds.type
		LEFT JOIN profile profile on ds.profile = profile.id
		where ds.cdn_id = $1`

		rows, err := db.Query(query, s.CdnID)
		if err != nil {
			return dsList, err
		}
		defer rows.Close()
	} else {
		query = `SELECT
		ds.id as id,
		ds.xml_id as xml_id,
		ds.org_server_fqdn as org_server_fqdn,
		type.name as type,
		ds.qstring_ignore as qstring_ignore,
		ds.origin_shield as origin_shield,
		ds.multi_site_origin as multi_site_origin,
		ds.profile as profile_id
		FROM deliveryservice ds
		JOIN type type on type.id = ds.type
		JOIN deliveryservice_server dss on dss.deliveryservice = ds.id
		LEFT JOIN profile profile on ds.profile = profile.id
		where dss.server = $1`

		rows, err := db.Query(query, s.ID)
		if err != nil {
			return dsList, err
		}
		defer rows.Close()
	}

	j := 0
	for rows.Next() {
		dsData := DSData{}

		var id sql.NullInt64
		var xmlID sql.NullString
		var orgServerFQDN sql.NullString
		var dsType sql.NullString
		var dsQstringIgnore sql.NullInt64
		var originShield sql.NullString
		var multiSiteOrigin sql.NullBool
		var profileID sql.NullInt64

		if err := rows.Scan(&id, &xmlID, &orgServerFQDN, &dsType, dsQstringIgnore, &originShield, &multiSiteOrigin, &profileID); err != nil {
			return dsList, err
		}

		if profileID {
			qsQuery := `SELECT
			p.value as value
			from profile_parameter pp 
			JOIN parameter p on pp.parameter = p.id
			where pp.profile = $1
			and p.name = 'psel.qstring_handling'`

			qsErr := db.QueryRow(qsQuery, profileID).Scan(&dsData.ParentQstringHandling)
			if qsErr != nil {
				return dsList, qsErr
			}

			msoQuery := `SELECT
			p.value as value,
			p.name as name,
			p.config_file as config_file
			from profile_parameter pp 
			JOIN parameter p on pp.parameter = p.id
			where pp.profile = $1
			and p.name like 'mso.%'`

			msoRows, msoErr := db.Query(msoQuery, profileID)
			if msoErr != nil {
				return dsList, msoErr
			}
			defer msoRows.Close()

			i := 0
			for msoRows.Next() {
				p := Parameters{}
				if pErr := msoRows.Scan(&p.Value, &p.Name, &p.ConfigFile); err != nil {
					return dsList, pErr
				}
				dsData.Params[i] = p
				i++
			}
		}

		dsData.ID = id
		dsData.XMLId = xmlID
		dsData.OrgServerFQDN = orgServerFQDN
		dsData.Type = dsType
		dsData.QStringIgnore = dsQstringIgnore
		dsData.OriginShield = originShield
		dsData.MultiSiteOrigin = multiSiteOrigin
		if profileID {
			dsData.Profile = profileID
		}

		dsList.DeliveryServices[j] = dsData
		j++
	}
	return dsList, nil
}

func headerComment(db *sql.DB, name string) (string, error) {
	nameString, err := nameURLString(db)
	if err != nil {
		return "", err
	}
	t := time.Now()
	date := t.Format("2006-01-02 15:04:05 MST")
	text := "# DO NOT EDIT - Generated for " + name + " by " + nameString + " at " + date + "\n"
	return text, nil
}

func nameURLString(db *sql.DB) (string, error) {
	queryToolName := `select value from Parameter where name = 'tm.toolname' and config_file = 'global' limit 1`

	toolName := ""
	err := db.QueryRow(queryToolName).Scan(&toolName)
	if err != nil {
		return "", err
	}

	queryURL := `select value from Parameter where name = 'tm.url' and config_file = 'global' limit 1`

	toolURL := ""
	err = db.QueryRow(queryURL).Scan(&toolURL)
	if err != nil {
		return "", err
	}
	nameString := ""
	nameString = toolName + " (" + toolURL + ")"
	return nameString, nil
}

func buildServerObj(db *sql.DB, hostName string) (ServerObj, error) {
	query := `SELECT 
	sv.id as id,
	sv.domain_name as domain_name,
	sv.tcp_port as tcp_port,
	sv.cachegroup as cachegroup_id,
	cachegroup.name as cachegroup,
	status.name as status,
	sv.last_updated as last_updated,
	profile.name as profile,
	sv.profile as profile_id,
	sv.upd_pending as upd_pending,
	parameter.value as ats_version,
	type.name as type,
	cdn.name as cdn,
	sv.cdn_id as cdn_id
	from server sv
	JOIN cachegroup cachegroup on cachegroup.id = sv.cachegroup
	JOIN status status on status.id = sv.status
	JOIN cdn cdn ON cdn.id = sv.cdn_id
	JOIN type type ON type.id = sv.type
	JOIN profile profile ON profile.id = sv.profile
	JOIN profile_parameter pp on profile.id = pp.profile
	JOIN parameter parameter on pp.parameter = parameter.id
	where sv.host_name = $1
	and parameter.config_file = 'package' 
	and parameter.name = 'trafficserver'`

	s := ServerObj{}
	s.HostName = hostName
	err := db.QueryRow(query, hostName).Scan(&s.ID, &s.DomainName, &s.TCPPort, &s.CachegroupID, &s.Cachegroup, &s.Status, &s.LastUpdated, &s.Profile, &s.ProfileID, &s.UpdatePending, &s.ATSVersion, &s.Type, &s.CDN, &s.CDNID)
	if err != nil {
		return s, err
	}

	if i := strings.Index(s.ATSVersion, "."); i >= 0 {
		s.ATSVersion = s.ATSVersion[:i]
	}

	return s, nil
}
