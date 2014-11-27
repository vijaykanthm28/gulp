package main

import (
    "github.com/tsuru/config"
    "github.com/megamsys/libgo/etcd"
    log "code.google.com/p/log4go"
	"os"
	"encoding/json"
	"os/signal"
	"regexp"
	"syscall"
	"time"
	"fmt"
	"github.com/megamsys/gulp/policies/bind"
	"github.com/megamsys/libgo/amqp"
	"github.com/megamsys/libgo/db"
	"github.com/megamsys/gulp/cmd/gulpd/queue"
	"github.com/megamsys/gulp/policies/ha"
	"github.com/megamsys/gulp/policies"
	"net"
	"net/url"
)

const (
	// queue actions
	runningApp = "running"
	startApp   = "start"
	stopApp    = "stop"
	buildApp   = "build"
	restartApp = "restart"
	addonApp   = "addon"
	queueName  = "gulpd-app"
)

var (
	signalChannel chan<- os.Signal
	nameRegexp    = regexp.MustCompile(`^[a-z][a-z0-9-]{0,62}$`)
)

func init() {
	bind.Init()
	ha.Init()
}

func RunServer(dry bool) {
	log.Info("Gulpd starting at %s", time.Now())
	signalChannel := make(chan os.Signal, 1)
	signal.Notify(signalChannel, syscall.SIGINT)
//	handler().Start() 
    Checker()
    name, _ := config.GetString("name")
    Watcher(name)
    
    updatename, _ := config.GetString("update_queue")
   Watcher(updatename)
    
    docker, _ := config.GetString("docker_queue")
   Watcher(docker)
    
	log.Info("Gulpd at your service.")
	updateStatus()
	<-signalChannel
	log.Info("Gulpd killed |_|.")
}

func StopServer(bark bool) {
	log.Info("Gulpd stopping at %s", time.Now())
	//handler().Stop()
	close(signalChannel)
	log.Info("Gulpd finished |-|.")
}

func Checker() {
	log.Info("Dialing Rabbitmq.......")
	factor, err := amqp.Factory()
	if err != nil {
		log.Error("Failed to get the queue instance: %s", err)
	}
	
	conn, connerr := factor.Dial()
    log.Debug("connection %v", conn)
    log.Debug("connection error %v", connerr)
    if connerr != nil {
    	 fmt.Fprintf(os.Stderr, "Error: %v\n Please start Rabbitmq service.\n", connerr)
         os.Exit(1)
    }
    log.Info("Rabbitmq connected")
    
    log.Info("Dialing Riak.......")
 
	 rconn, rerr := db.Conn("connection")
	 if rerr != nil {
		 fmt.Fprintf(os.Stderr, "Error: %v\n Please start Riak service.\n", connerr)
         os.Exit(1)
	 }

	 data := "sampledata"
	 ferr := rconn.StoreObject("sampleobject", data)
	 if ferr != nil {
	 	 fmt.Fprintf(os.Stderr, "Error: %v\n Please start Riak service.\n", ferr)
         os.Exit(1)
	 }
	 defer rconn.Close()
    log.Info("Riak connected")
	
}

/*func updateStatus() {
	path, _ := config.GetString("etcd:path")
	c := etcd.NewClient(path+"/")
	conn, connerr := c.Dial("tcp", "127.0.0.1:4001")
    log.Debug("client %v", c)
    log.Debug("connection %v", conn)
    log.Debug("connection error %v", connerr)
    
    if conn != nil {
	dir, _ := config.GetString("etcd:directory")
	id, _ := config.GetString("id")
	name, _ := config.GetString("name")
	mapD := map[string]string{"id": id, "status": "RUNNING"}
    mapB, _ := json.Marshal(mapD)
   	
   	log.Info(c)
   	log.Info(name)
   	log.Info(dir)
   	log.Info(mapB)
   	
	//c := etcd.NewClient(nil)
	_, err := c.Create("/"+dir+"/"+name, string(mapB))
  
	if err != nil {
		log.Error("===========",err)
	}
   } else {
  	 fmt.Fprintf(os.Stderr, "Error: %v\n Please start etcd deamon.\n", connerr)
         os.Exit(1)
  }
}*/

func updateStatus() {
	path, _ := config.GetString("etcd:path")
	c := etcd.NewClient([]string{path})
	success := c.SyncCluster()
	if !success {
		log.Debug("cannot sync machines")
	}

	for _, m := range c.GetCluster() {
		u, err := url.Parse(m)
		if err != nil {
			log.Debug(err)
		}
		if u.Scheme != "http" {
			log.Debug("scheme must be http")
		}
        log.Info(u.Host)
		host, _, err := net.SplitHostPort(u.Host)
		if err != nil {
			log.Debug(err)
		}
		if host != "127.0.0.1" {
			log.Debug("Host must be 127.0.0.1")
		}
	}
	etcdNetworkPath, _ := config.GetString("etcd:networkpath")
    conn, connerr := c.Dial("tcp", etcdNetworkPath)
    log.Debug("client %v", c)
    log.Debug("connection %v", conn)
    log.Debug("connection error %v", connerr)
    
    if conn != nil {
	dir, _ := config.GetString("etcd:directory")
	id, _ := config.GetString("id")
	name, _ := config.GetString("name")
	mapD := map[string]string{"id": id, "status": "RUNNING"}
    mapB, _ := json.Marshal(mapD)	
  
   	
	//c := etcd.NewClient(nil)
	_, err := c.Create("/"+dir+"/"+name, string(mapB))
  
	if err != nil {
		log.Error("===========",err)
	}
	
	aid, _ := config.GetString("id")
	UpdateRiakStatus(aid)
	
   } else {
  	 fmt.Fprintf(os.Stderr, "Error: %v\n Please start etcd deamon.\n", connerr)
         os.Exit(1)
  }
}


func Watcher(queue_name string) {    
	    queueserver1 := queue.NewServer(queue_name)
		go queueserver1.ListenAndServe()
}

func UpdateRiakStatus(id string) error {
	asm := &policies.Assembly{}
	conn, err := db.Conn("assembly")
	if err != nil {	
		return err
	}	
	//appout := &Requests{}
	ferr := conn.FetchStruct(id, asm)
	if ferr != nil {	
		return ferr
	}	
	
	update := policies.Assembly{
		Id:           asm.Id, 
        JsonClaz:      asm.JsonClaz, 
        Name:          asm.Name, 
        Components:    asm.Components ,
        Policies:      asm.Policies,
        Inputs:        asm.Inputs,
        Operations:    asm.Operations,
        Outputs:       asm.Outputs,
        Status:        "Running",
        CreatedAt:     asm.CreatedAt,
	}
	err = conn.StoreStruct(asm.Id, &update)
	
	return err
}


