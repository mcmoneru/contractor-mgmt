package main

import (
	"encoding/json"

	v1 "github.com/decred/contractor-mgmt/cmswww/api/v1"
	pd "github.com/decred/politeia/politeiad/api/v1"
)

type MDStreamChanges struct {
	Version        uint             `json:"version"`        // Version of the struct
	AdminPublicKey string           `json:"adminpublickey"` // Identity of the administrator
	NewStatus      pd.RecordStatusT `json:"newstatus"`      // NewStatus
	Timestamp      int64            `json:"timestamp"`      // Timestamp of the change
}

type BackendInvoiceMetadata struct {
	Version   uint64 `json:"version"` // BackendInvoiceMetadata version
	Month     uint16 `json:"month"`
	Year      uint16 `json:"year"`
	Timestamp int64  `json:"timestamp"` // Last update of invoice
	PublicKey string `json:"publickey"` // Key used for signature.
	Signature string `json:"signature"` // Signature of merkle root
}

func convertInvoiceStatusFromWWW(s v1.InvoiceStatusT) pd.RecordStatusT {
	switch s {
	case v1.InvoiceStatusNotFound:
		return pd.RecordStatusNotFound
	case v1.InvoiceStatusNotReviewed:
		return pd.RecordStatusNotReviewed
	case v1.InvoiceStatusRejected:
		return pd.RecordStatusCensored
	case v1.InvoiceStatusApproved:
		return pd.RecordStatusPublic
	}
	return pd.RecordStatusInvalid
}

func convertInvoiceFileFromWWW(f *v1.File) []pd.File {
	return []pd.File{{
		Name:    "invoice.csv",
		MIME:    f.MIME,
		Digest:  f.Digest,
		Payload: f.Payload,
	}}
}

func convertInvoiceCensorFromWWW(f v1.CensorshipRecord) pd.CensorshipRecord {
	return pd.CensorshipRecord{
		Token:     f.Token,
		Merkle:    f.Merkle,
		Signature: f.Signature,
	}
}

// convertInvoiceFromWWW converts a www invoice to a politeiad record.  This
// function should only be used in tests. Note that convertInvoiceFromWWW can not
// emulate MD properly.
func convertInvoiceFromWWW(p v1.InvoiceRecord) pd.Record {
	return pd.Record{
		Status:    convertInvoiceStatusFromWWW(p.Status),
		Timestamp: p.Timestamp,
		Metadata: []pd.MetadataStream{{
			ID:      pd.MetadataStreamsMax + 1, // fail deliberately
			Payload: "invalid payload",
		}},
		Files:            convertInvoiceFileFromWWW(p.File),
		CensorshipRecord: convertInvoiceCensorFromWWW(p.CensorshipRecord),
	}
}

func convertInvoicesFromWWW(p []v1.InvoiceRecord) []pd.Record {
	pr := make([]pd.Record, 0, len(p))
	for _, v := range p {
		pr = append(pr, convertInvoiceFromWWW(v))
	}
	return pr
}

///////////////////////////////
func convertInvoiceStatusFromPD(s pd.RecordStatusT) v1.InvoiceStatusT {
	switch s {
	case pd.RecordStatusNotFound:
		return v1.InvoiceStatusNotFound
	case pd.RecordStatusNotReviewed:
		return v1.InvoiceStatusNotReviewed
	case pd.RecordStatusCensored:
		return v1.InvoiceStatusRejected
	case pd.RecordStatusPublic:
		return v1.InvoiceStatusApproved
	}
	return v1.InvoiceStatusInvalid
}

func convertInvoiceFileFromPD(files []pd.File) *v1.File {
	if len(files) == 0 {
		return nil
	}

	return &v1.File{
		MIME:    files[0].MIME,
		Digest:  files[0].Digest,
		Payload: files[0].Payload,
	}
}

func convertInvoiceCensorFromPD(f pd.CensorshipRecord) v1.CensorshipRecord {
	return v1.CensorshipRecord{
		Token:     f.Token,
		Merkle:    f.Merkle,
		Signature: f.Signature,
	}
}

func convertInvoiceFromInventoryRecord(r *inventoryRecord, userPubkeys map[string]string) v1.InvoiceRecord {
	invoice := convertInvoiceFromPD(r.record)

	// Set the most up-to-date status.
	for _, v := range r.changes {
		invoice.Status = convertInvoiceStatusFromPD(v.NewStatus)
	}

	// Set the user id.
	var ok bool
	invoice.UserID, ok = userPubkeys[invoice.PublicKey]
	if !ok {
		log.Errorf("user not found for public key %v, for invoice %v",
			invoice.PublicKey, invoice.CensorshipRecord.Token)
	}

	return invoice
}

func convertInvoiceFromPD(p pd.Record) v1.InvoiceRecord {
	md := &BackendInvoiceMetadata{}
	for _, v := range p.Metadata {
		if v.ID != mdStreamGeneral {
			continue
		}
		err := json.Unmarshal([]byte(v.Payload), md)
		if err != nil {
			log.Errorf("could not decode metadata '%v' token '%v': %v",
				p.Metadata, p.CensorshipRecord.Token, err)
			break
		}
	}

	return v1.InvoiceRecord{
		Status:           convertInvoiceStatusFromPD(p.Status),
		Timestamp:        md.Timestamp,
		Month:            md.Month,
		Year:             md.Year,
		PublicKey:        md.PublicKey,
		Signature:        md.Signature,
		File:             convertInvoiceFileFromPD(p.Files),
		CensorshipRecord: convertInvoiceCensorFromPD(p.CensorshipRecord),
	}
}

func convertErrorStatusFromPD(s int) v1.ErrorStatusT {
	switch pd.ErrorStatusT(s) {
	case pd.ErrorStatusInvalidFileDigest:
		return v1.ErrorStatusInvalidFileDigest
	case pd.ErrorStatusInvalidBase64:
		return v1.ErrorStatusInvalidBase64
	case pd.ErrorStatusInvalidMIMEType:
		return v1.ErrorStatusInvalidMIMEType
	case pd.ErrorStatusUnsupportedMIMEType:
		return v1.ErrorStatusUnsupportedMIMEType
	case pd.ErrorStatusInvalidRecordStatusTransition:
		return v1.ErrorStatusInvalidInvoiceStatusTransition

		// These cases are intentionally omitted because
		// they are indicative of some internal server error,
		// so ErrorStatusInvalid is returned.
		//
		//case pd.ErrorStatusInvalidRequestPayload
		//case pd.ErrorStatusInvalidChallenge
	}
	return v1.ErrorStatusInvalid
}