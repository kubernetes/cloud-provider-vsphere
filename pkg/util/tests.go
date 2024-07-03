package util

import (
	"reflect"
	testing2 "testing"

	"k8s.io/apimachinery/pkg/util/diff"
	"k8s.io/client-go/testing"
)

// CheckActions verifies that expected and actual action slices are equal
func CheckActions(expected, actual []testing.Action, t *testing2.T) {
	for i, actualAction := range actual {
		if len(expected) < i+1 {
			t.Errorf("%d unexpected actions: %+v", len(actual)-len(expected), actual[i:])
			break
		}

		expectedAction := expected[i]
		CheckAction(expectedAction, actualAction, t)
	}
	if len(expected) > len(actual) {
		t.Errorf("%d additional expected actions:%+v", len(expected)-len(actual), expected[len(actual):])
	}
}

// CheckAction verifies that expected and actual actions are equal and both have
// same attached resources
func CheckAction(expected, actual testing.Action, t *testing2.T) {
	if !(expected.Matches(actual.GetVerb(), actual.GetResource().Resource) && actual.GetSubresource() == expected.GetSubresource()) {
		t.Errorf("Expected\n\t%#v\ngot\n\t%#v", expected, actual)
		return
	}

	if reflect.TypeOf(actual) != reflect.TypeOf(expected) {
		t.Errorf("Action has wrong type. Expected: %t. Got: %t", expected, actual)
		return
	}

	switch a := actual.(type) {
	case testing.CreateActionImpl:
		e, _ := expected.(testing.CreateActionImpl)
		expObject := e.GetObject()
		object := a.GetObject()

		if !reflect.DeepEqual(expObject, object) {
			t.Errorf("Action %s %s has wrong object\nDiff:\n %s",
				a.GetVerb(), a.GetResource().Resource, diff.ObjectGoPrintSideBySide(expObject, object))
		}
	case testing.UpdateActionImpl:
		e, _ := expected.(testing.UpdateActionImpl)
		expObject := e.GetObject()
		object := a.GetObject()

		if !reflect.DeepEqual(expObject, object) {
			t.Errorf("Action %s %s has wrong object\nDiff:\n %s",
				a.GetVerb(), a.GetResource().Resource, diff.ObjectGoPrintSideBySide(expObject, object))
		}
	case testing.PatchActionImpl:
		e, _ := expected.(testing.PatchActionImpl)
		expPatch := e.GetPatch()
		patch := a.GetPatch()

		if !reflect.DeepEqual(expPatch, patch) {
			t.Errorf("Action %s %s has wrong patch\nDiff:\n %s",
				a.GetVerb(), a.GetResource().Resource, diff.ObjectGoPrintSideBySide(expPatch, patch))
		}
	case testing.DeleteActionImpl:
		e, _ := expected.(testing.DeleteActionImpl)
		expName := e.GetName()
		name := a.GetName()
		if expName != name {
			t.Errorf("Action %s %s has wrong name. Expected: %s. Got: %s",
				a.GetVerb(), a.GetResource().Resource, expName, name)
		}
	default:
		t.Errorf("Uncaptured Action %s %s, you should explicitly add a case to capture it",
			actual.GetVerb(), actual.GetResource().Resource)
	}
}
