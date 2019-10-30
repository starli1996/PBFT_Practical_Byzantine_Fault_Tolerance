package network

import (
	"github.com/bigpicturelabs/consensusPBFT/pbft/consensus"
	"fmt"

	"sync/atomic"
	"unsafe"
)

func (node *Node) StartViewChange() {
	var vcs *consensus.ViewChangeState

	// Start_ViewChange
	LogStage("ViewChange", false) 

	// Create nextviewid.
	var nextviewid = node.View.ID + 1
	vcs = node.ViewChangeState
	for vcs == nil {
		vcs = consensus.CreateViewChangeState(node.MyInfo.NodeID, len(node.NodeTable), nextviewid, node.StableCheckPoint)
		//Create ViewChangeState if ViewChangeState is nil.
		if !atomic.CompareAndSwapPointer((*unsafe.Pointer)(unsafe.Pointer(&node.ViewChangeState)), unsafe.Pointer(nil), unsafe.Pointer(vcs)) {
			vcs = node.ViewChangeState
		}
	}

	// a set of PreprepareMsg and PrepareMsgs for veiwchange.
	setp := make(map[int64]*consensus.SetPm)
	setc := make(map[string]*consensus.CheckPointMsg)
	
	node.StatesMutex.RLock()
	for seqID, state := range node.States {
		var setPm consensus.SetPm
		setPm.PrePrepareMsg = state.GetPrePrepareMsg()
		setPm.PrepareMsgs = state.GetPrepareMsgs()
		setp[seqID] = &setPm
	}
	
	fmt.Println("node.StableCheckPoint : ", node.StableCheckPoint)
	setc = node.CheckPointMsgsLog[node.StableCheckPoint]
	fmt.Println("setc",setc)


	node.StatesMutex.RUnlock()

	// Create ViewChangeMsg.
	viewChangeMsg, err := vcs.CreateViewChangeMsg(setp, setc)

	if err != nil {
		node.MsgError <- []error{err}
		return
	}

	node.Broadcast(viewChangeMsg, "/viewchange")
	fmt.Println("Breadcast viewchange")
	LogStage("ViewChange", true)

	node.GetViewChange(viewChangeMsg)
}

func (node *Node) NewView(newviewMsg *consensus.NewViewMsg) {
	LogMsg(newviewMsg)

	node.Broadcast(newviewMsg, "/newview")
	LogStage("NewView", true)

	node.ViewChangeState = nil
	
	node.IsViewChanging = false
}

func (node *Node) GetViewChange(viewchangeMsg *consensus.ViewChangeMsg) error {
	var vcs *consensus.ViewChangeState

	LogMsg(viewchangeMsg)

	if node.View.ID >= viewchangeMsg.NextViewID {
		return nil
	}

	// Create nextviewid
	var nextviewid =  node.View.ID + 1
	vcs = node.ViewChangeState
	for vcs == nil {
		vcs = consensus.CreateViewChangeState(node.MyInfo.NodeID, len(node.NodeTable), nextviewid, node.StableCheckPoint)
		// Create ViewChangeState if ViewChangeState is nil.
		if !atomic.CompareAndSwapPointer((*unsafe.Pointer)(unsafe.Pointer(&node.ViewChangeState)), unsafe.Pointer(nil), unsafe.Pointer(vcs)) {
			vcs = node.ViewChangeState
		}
	}

	newView, err := vcs.ViewChange(viewchangeMsg)
	if err != nil {
		return err
	}

	var nextPrimary = node.getPrimaryInfoByID(nextviewid)

	if newView != nil && node.MyInfo == nextPrimary {
		// Change View and Primary.
		node.updateView(newView.NextViewID)

		// Search min_s the sequence number of the latest stable checkpoint and
		// max_s the highest sequence number in a prepare message in V.
		var min_s int64 
		min_s = 0
		var max_s int64
		max_s = 0

		fmt.Println("***********************N E W V I E W***************************")
		for _, vcm := range newView.SetViewChangeMsgs {
			if min_s < vcm.StableCheckPoint {
				min_s = vcm.StableCheckPoint
			}
			for  _, prepareSet := range vcm.SetP {
				for _, prepareMsg := range prepareSet.PrepareMsgs {
					if max_s < prepareMsg.SequenceID {
						max_s = prepareMsg.SequenceID
					}
				}
			}
		}

		fmt.Println("min_s ", min_s, "max_s", max_s)
		fmt.Println("newView")

		LogStage("NewView", false)
		node.NewView(newView)

	}

	return nil
}

func (node *Node) GetNewView(msg *consensus.NewViewMsg) error{

	fmt.Printf("<<<<<<<<<<<<<<<<NewView>>>>>>>>>>>>>>>>: %d by %s\n", msg.NextViewID, msg.NodeID)

	//Change View and Primary
	node.updateView(msg.NextViewID)

	node.ViewChangeState = nil

	node.IsViewChanging = false

	return nil
}

func (node *Node) updateView(viewID int64) {
	node.View.ID = viewID
	node.View.Primary = node.getPrimaryInfoByID(viewID)
}

func (node *Node) isMyNodePrimary() bool {
	return node.MyInfo.NodeID == node.View.Primary.NodeID
}

func (node *Node) getPrimaryInfoByID(viewID int64) *NodeInfo {
	viewIdx := viewID % int64(len(node.NodeTable))
	return node.NodeTable[viewIdx]
}
