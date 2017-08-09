package runner

// This file contains the implementations of various functions for accessing
// and using google firebase.  Firebase is being used by TensorFlow Studio
// to contextual information about tasks it has requested be executed, via
// Google PubSub

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"time"

	"github.com/zabawaba99/firego"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"

	"github.com/mgutz/logxi/v1"
)

var (
	authDB = flag.String("firebase-account-file", "", "The file in which the Google Service Account authrization details are stored for Firebase")
	logger = log.New("runner.model")
)

func init() {
	*authDB = os.Getenv("HOME") + "/.ssh/google-firebase-auth.json"
}

type FirebaseDB struct {
	fb        *firego.Firebase
	projectID string
}

func NewDatabase(projectID string) (db *FirebaseDB, err error) {

	info, err := os.Stat(*authDB)
	if err != nil {
		return nil, err
	}
	if 0600 != info.Mode() {
		return nil, fmt.Errorf(`file permissions for %s are too liberal, permissions should be 0600, 
		use the shell command 'chmod 0600 %s' to fix this`, *authDB, *authDB)
	}

	d, err := ioutil.ReadFile(*authDB)
	if err != nil {
		return nil, err
	}

	conf, err := google.JWTConfigFromJSON(d, "https://www.googleapis.com/auth/userinfo.email",
		"https://www.googleapis.com/auth/firebase.database")
	if err != nil {
		return nil, err
	}

	db = &FirebaseDB{
		projectID: projectID,
	}

	db.fb = firego.New(fmt.Sprintf("https://%s.firebaseio.com", projectID), conf.Client(oauth2.NoContext))

	firego.TimeoutDuration = 5 * time.Second

	return db, nil
}

func (fb *FirebaseDB) GetExperiment(experiment string) (result *TFSMetaData, err error) {

	v := map[string]interface{}{}

	err = fb.fb.Child("experiments").Child(experiment).Value(&v)
	if err != nil {
		return nil, err
	}

	for k, _ := range v {
		item := map[string]interface{}{}
		fb.fb.Child("experiments").Child(experiment).Child(k).Value(&item)

	}

	json, err := json.MarshalIndent(v, "", "    ")
	if err != nil {
		return nil, err
	}

	return UnmarshalTFSMetaData(json)
}

type Artifact struct {
	Archive string
	Mutable bool
}

func (fb *FirebaseDB) GetManifest(experiment string) (manifest map[string]Artifact, err error) {

	artifacts := map[string]interface{}{}

	err = fb.fb.Child("experiments").Child(experiment).Child("artifacts").Value(&artifacts)
	if err != nil {
		return nil, err
	}

	manifest = map[string]Artifact{}

	for name, artifact := range artifacts {
		archive, pathPresent := artifact.(map[string]interface{})["key"]
		mutable, mutePresent := artifact.(map[string]interface{})["mutable"]
		if pathPresent && mutePresent {
			manifest[name] = Artifact{
				Archive: archive.(string),
				Mutable: mutable.(bool),
			}
		}
	}

	return manifest, nil
}