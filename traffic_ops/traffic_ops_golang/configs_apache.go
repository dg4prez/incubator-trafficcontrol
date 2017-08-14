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

func atsMetadata(db *sql.DB) RegexHandlerFunc {
	return func(w http.ResponseWriter, r *http.Request, p ParamMap) {
		handleErr := func(err error, status int) {
			log.Errorf("%v %v\n", r.RemoteAddr, err)
			w.WriteHeader(status)
			fmt.Fprintf(w, http.StatusText(status))
		}

		hostName := p["host"]

		s, err := buildServerObj(db, hostName)
		if err != nil {
			log.Errorf("Error: %v\n", err)
			handleErr(err, http.StatusInternalServerError)
			return
		}

		text := "I'm metadata for " + s.HostName + "!"

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
		default:
			text = "I'm metadata!"
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

func parentConfig(db *sql.DB, s ServerObj, header string) (string, error) {
	text := ""
	dsList, err := deliveryServiceData(db, s)
	if err != nil {
		return text, err
	}

	if strings.HasPrefix(s.Type, "MID") {
		log.Debugln("mid found")
		for _, ds := range dsList.DeliveryServices {

			if ds.Profile != 0 {
				log.Debugln("found a profile id of ", ds.Profile)
				if ds.MsoAlgorithm != "" {
					log.Debugln("found an algorithm of ", ds.MsoAlgorithm)
				}

			}
			text = text + ds.OrgServerFQDN + "         " + ds.XMLId + "      " + ds.ParentQstringHandling + "\n"
		}
	} else {
		log.Debugln("not a mid")
		for _, ds := range dsList.DeliveryServices {
			text = text + ds.OrgServerFQDN + "         " + ds.XMLId + "      " + ds.ParentQstringHandling + "\n"
		}
	}

	text = header + s.ATSVersion + s.Type + text
	return text, nil
}

// DSList struct
type DSList struct {
	DeliveryServices []DSData
}

// DSData struct
type DSData struct {
	ID                                 int
	XMLId                              string
	OrgServerFQDN                      string
	Type                               string
	QStringIgnore                      int
	OriginShield                       string
	MultiSiteOrigin                    bool
	Profile                            int
	ParentQstringHandling              string
	MsoAlgorithm                       string
	MsoParentRetry                     string
	MsoUnavailableServerRetryResponses string
	MsoMaxSimpleRetries                string
	MsoMaxUnavailableServerRetries     string
}

// Parameters struct
type Parameters struct {
	ConfigFile string
	Name       string
	Value      string
}

func deliveryServiceData(db *sql.DB, s ServerObj) (DSList, error) {
	dsList := DSList{}
	query := ""
	id := 0
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

		id = s.CDNID
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

		id = s.ID
	}
	rows, err := db.Query(query, id)
	if err != nil {
		return dsList, err
	}
	defer rows.Close()

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

		if err := rows.Scan(&id, &xmlID, &orgServerFQDN, &dsType, &dsQstringIgnore, &originShield, &multiSiteOrigin, &profileID); err != nil {
			log.Debugln("dsdata scan:", err)
			return dsList, err
		}

		if profileID.Valid {
			dsData.Profile = int(profileID.Int64)

			parentQstringHandling, err := SpecificParameterSearch(db, int(profileID.Int64), "parent.config", "psel.qstring_handling")
			if err == sql.ErrNoRows {
				// Don't do anything. This is like, normal, man.
			} else if err != nil {
				return dsList, fmt.Errorf("Querying and scanning parentQstringHandling: %v", err)
			} else {
				dsData.ParentQstringHandling = parentQstringHandling.String
			}

			msoAlgorithm, err := SpecificParameterSearch(db, int(profileID.Int64), "parent.config", "mso.algorithm")
			if err == sql.ErrNoRows {
				// Don't do anything. This is like, normal, man.
			} else if err != nil {
				return dsList, fmt.Errorf("Querying and scanning mso.algorithm: %v", err)
			} else {
				dsData.MsoAlgorithm = msoAlgorithm.String
			}

			msoParentRetry, err := SpecificParameterSearch(db, int(profileID.Int64), "parent.config", "mso.algorithm")
			if err == sql.ErrNoRows {
				// Don't do anything. This is like, normal, man.
			} else if err != nil {
				return dsList, fmt.Errorf("Querying and scanning mso.parent_retry: %v", err)
			} else {
				dsData.MsoParentRetry = msoParentRetry.String
			}

			msoUnavailableServerRetryResponses, err := SpecificParameterSearch(db, int(profileID.Int64), "parent.config", "mso.algorithm")
			if err == sql.ErrNoRows {
				// Don't do anything. This is like, normal, man.
			} else if err != nil {
				return dsList, fmt.Errorf("Querying and scanning mso.parent_retry: %v", err)
			} else {
				dsData.MsoUnavailableServerRetryResponses = msoUnavailableServerRetryResponses.String
			}

			msoMaxSimpleRetries, err := SpecificParameterSearch(db, int(profileID.Int64), "parent.config", "mso.algorithm")
			if err == sql.ErrNoRows {
				// Don't do anything. This is like, normal, man.
			} else if err != nil {
				return dsList, fmt.Errorf("Querying and scanning mso.parent_retry: %v", err)
			} else {
				dsData.MsoMaxSimpleRetries = msoMaxSimpleRetries.String
			}

			msoMaxUnavailableServerRetries, err := SpecificParameterSearch(db, int(profileID.Int64), "parent.config", "mso.algorithm")
			if err == sql.ErrNoRows {
				// Don't do anything. This is like, normal, man.
			} else if err != nil {
				return dsList, fmt.Errorf("Querying and scanning mso.parent_retry: %v", err)
			} else {
				dsData.MsoMaxUnavailableServerRetries = msoMaxUnavailableServerRetries.String
			}
			/*msoQuery := `SELECT
			p.value as value,
			p.name as name,
			p.config_file as config_file
			from profile_parameter pp
			JOIN parameter p on pp.parameter = p.id
			where pp.profile = $1
			and p.name like 'mso.%'`

			msoRows, msoErr := db.Query(msoQuery, profileID)
			if msoErr != nil {
				return dsList, fmt.Errorf("Querying mso profile params: %v", msoErr)
			}
			defer msoRows.Close()

			for msoRows.Next() {
				p := Parameters{}
				if pErr := msoRows.Scan(&p.Value, &p.Name, &p.ConfigFile); pErr != nil {
					log.Debugln("parameter scan", pErr)
					return dsList, fmt.Errorf("Scanning mso profile params: %v", pErr)
				}
				dsData.Params = append(dsData.Params, p)
			}*/
		}

		dsData.ID = int(id.Int64)
		dsData.XMLId = xmlID.String
		dsData.OrgServerFQDN = orgServerFQDN.String
		dsData.Type = dsType.String
		dsData.QStringIgnore = int(dsQstringIgnore.Int64)
		dsData.OriginShield = originShield.String
		dsData.MultiSiteOrigin = multiSiteOrigin.Bool
		dsList.DeliveryServices = append(dsList.DeliveryServices, dsData)
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
	text := "# DO NOT EDIT - Generated with the GO API for " + name + " by " + nameString + " at " + date + "\n"
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
	and parameter.name = 'trafficserver'
	limit 1`

	s := ServerObj{}
	s.HostName = hostName
	err := db.QueryRow(query, hostName).Scan(&s.ID, &s.DomainName, &s.TCPPort, &s.CachegroupID, &s.Cachegroup, &s.Status, &s.LastUpdated, &s.Profile, &s.ProfileID, &s.UpdatePending, &s.ATSVersion, &s.Type, &s.CDN, &s.CDNID)
	if err != nil {
		log.Debugln(err)
		return s, fmt.Errorf("Querying and scanning server data: %v", err)
	}

	if i := strings.Index(s.ATSVersion, "."); i >= 0 {
		s.ATSVersion = s.ATSVersion[:i]
	}

	return s, nil
}

func parentData(db *sql.DB, s ServerObj) ([]string, error) {
	var parentData []string
	var primaryParentCGs []int
	var secondaryParentCGs []int

	if strings.HasPrefix(s.Type, "MID") {
		typeID, err := TypeID(db, "ORG_LOC")
		if err != nil {
			return parentData, err
		}

		//Get the cachegroup IDs

		query := `select id from cachegroup where type = $1`
		rows, err := db.Query(query, typeID)
		if err == sql.ErrNoRows {
			// Don't do anything. It just means we have no defined org_loc cachegroups.
		} else if err != nil {
			return parentData, fmt.Errorf("Searching for parent cachegroups: %v", err)
		} else {
			for rows.Next() {
				var id int
				if err = rows.Scan(&id); err != nil {
					return parentData, fmt.Errorf("Scanning parent cachegroups: %v", err)
				}
				primaryParentCGs = append(primaryParentCGs, id)
				log.Debugln(id)
			}
		}
	} else {
		var pID sql.NullInt64
		var sID sql.NullInt64

		//Get the primary cachegroup IDs

		query := `select parent_cachegroup_id from cachegroup where id = $1`
		err := db.QueryRow(query, s.CachegroupID).Scan(&pID)
		if err != nil {
			return parentData, fmt.Errorf("Searching for primary parent cachegroup: %v", err)
		}
		if pID.Valid {
			primaryParentCGs = append(primaryParentCGs, int(pID.Int64))
		}
		//Get the secondary cachegroup IDs

		query2 := `select parent_cachegroup_id from cachegroup where id = $1`
		err = db.QueryRow(query2, s.CachegroupID).Scan(&sID)
		if err != nil {
			return parentData, fmt.Errorf("Searching for primary parent cachegroup: %v", err)
		}
		if sID.Valid {
			secondaryParentCGs = append(secondaryParentCGs, int(sID.Int64))
		}
	}

	// Get the server's CDN domain
	var serverDomain string
	query3 := `select domain_name from cdn where id = $1`
	err := db.QueryRow(query3, s.CDNID).Scan(&serverDomain)
	if err != nil {
		return parentData, fmt.Errorf("Searching for server domain: %v", err)
	}
	return parentData, nil
}
