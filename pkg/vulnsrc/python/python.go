package python

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/aquasecurity/trivy-db/pkg/types"

	bolt "github.com/etcd-io/bbolt"

	"golang.org/x/xerrors"

	"github.com/aquasecurity/trivy-db/pkg/db"
	"github.com/aquasecurity/trivy-db/pkg/vulnsrc/vulnerability"
)

// https://github.com/pyupio/safety-db.git

const (
	pythonDir = "python-safety-db"
)

var (
	repoPath string
)

type AdvisoryDB map[string][]RawAdvisory

type RawAdvisory struct {
	ID       string
	Advisory string
	Cve      string
	Specs    []string
	Version  string `json:"v"`
}

type Advisory struct {
	Specs []string
}

type VulnSrc struct {
	dbc db.Operations
}

func NewVulnSrc() VulnSrc {
	return VulnSrc{
		dbc: db.Config{},
	}
}

func (vs VulnSrc) Update(dir string) (err error) {
	repoPath = filepath.Join(dir, pythonDir)
	if err := vs.update(repoPath); err != nil {
		return xerrors.Errorf("failed to update python vulnerabilities: %w", err)
	}
	return nil
}

func (vs VulnSrc) update(repoPath string) error {
	f, err := os.Open(filepath.Join(repoPath, "data", "insecure_full.json"))
	if err != nil {
		return xerrors.Errorf("failed to open the file: %w", err)
	}
	defer f.Close()

	// for detecting vulnerabilities
	var advisoryDB AdvisoryDB
	if err = json.NewDecoder(f).Decode(&advisoryDB); err != nil {
		return xerrors.Errorf("failed to decode JSON: %w", err)
	}

	// for displaying vulnerability detail
	err = vs.dbc.BatchUpdate(func(tx *bolt.Tx) error {
		if err := vs.commit(tx, advisoryDB); err != nil {
			return xerrors.Errorf("failed to save python vulnerabilities: %w", err)
		}
		return nil
	})
	if err != nil {
		return xerrors.Errorf("batch update failed: %w", err)
	}
	return nil
}

func (vs VulnSrc) commit(tx *bolt.Tx, advisoryDB AdvisoryDB) error {
	for pkgName, advisories := range advisoryDB {
		for _, advisory := range advisories {
			vulnerabilityID := advisory.Cve
			if vulnerabilityID == "" {
				vulnerabilityID = advisory.ID
			}

			// to detect vulnerabilities
			a := Advisory{Specs: advisory.Specs}
			err := vs.dbc.PutAdvisory(tx, vulnerability.PythonSafetyDB, pkgName, vulnerabilityID, a)
			if err != nil {
				return xerrors.Errorf("failed to save python advisory: %w", err)
			}

			// to display vulnerability detail
			vuln := types.VulnerabilityDetail{
				ID:    vulnerabilityID,
				Title: advisory.Advisory,
			}
			if err = vs.dbc.PutVulnerabilityDetail(tx, vulnerabilityID, vulnerability.PythonSafetyDB, vuln); err != nil {
				return xerrors.Errorf("failed to save python vulnerability detail: %w", err)
			}
		}
	}
	return nil
}
