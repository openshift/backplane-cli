package announcements

func HandoverAnnouncements(clusterKey string) error {
	clusterRecord, err := createClusterRecord(clusterKey)
	if err != nil {
		return err
	}

	err = relatedHandoverAnnouncements(clusterRecord)
	if err != nil {
		return err
	}
	return nil
}
