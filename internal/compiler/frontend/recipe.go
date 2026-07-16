package frontend

import "github.com/WindowsSov8forUs/sonolus-go/v2/internal/compiler/catalog"

func operationSet(values ...string) map[string]struct{} {
	result := make(map[string]struct{}, len(values))
	for _, value := range values {
		result[value] = struct{}{}
	}
	return result
}

var aggregateOperations = operationSet(
	"ease", "easing.linstep", "easing.smoothstep", "easing.smootherstep", "easing.stepStart", "easing.stepEnd",
	"control.terminate", "mode.isPlay", "mode.isWatch", "mode.isPreview", "mode.isTutorial", "mode.isPreprocessing",
	"touch.get", "entity.key", "time.previous", "time.offsetAdjusted", "screen.rect",
	"range.new", "range.length", "range.isEmpty", "range.mid", "range.contains", "range.containsRange",
	"range.add", "range.sub", "range.mul", "range.div", "range.intersect", "range.shrink", "range.expand",
	"range.lerp", "range.lerpClamped", "range.unlerp", "range.unlerpClamped", "range.clamp",
	"vec2.new", "vec2.unit", "vec2.add", "vec2.sub", "vec2.mul", "vec2.div", "vec2.mulVec", "vec2.divVec", "vec2.negate", "vec2.lerp", "vec2.lerpClamped", "vec2.dot", "vec2.magnitudeSquared",
	"vec2.magnitude", "vec2.angle", "vec2.orthogonal", "vec2.normalize", "vec2.normalizeOrZero", "vec2.rotate",
	"vec2.rotateAbout", "vec2.angleDiff", "vec2.signedAngleDiff",
	"rect.fromCenter", "rect.fromMargin", "rect.width", "rect.height", "rect.center", "rect.bl", "rect.tl", "rect.tr", "rect.br",
	"rect.top", "rect.right", "rect.bottom", "rect.left", "rect.translate", "rect.scale",
	"rect.scaleVec", "rect.scaleAbout", "rect.scaleCentered", "rect.expand", "rect.shrink", "rect.toQuad", "rect.contains",
	"quad.center", "quad.translate", "quad.scale", "quad.scaleVec", "quad.scaleAbout", "quad.scaleCentered",
	"quad.rotate", "quad.rotateAbout", "quad.rotateCentered", "quad.top", "quad.right", "quad.bottom",
	"quad.left", "quad.permute", "quad.contains",
	"transform.identity",
	"transform.translate", "transform.scale", "transform.rotate", "transform.compose", "transform.composeBefore",
	"transform.scaleAbout", "transform.rotateAbout", "transform.shearX", "transform.shearY",
	"transform.simplePerspectiveX", "transform.simplePerspectiveY", "transform.perspectiveX", "transform.perspectiveY",
	"transform.inversePerspectiveX", "transform.inversePerspectiveY", "transform.normalize", "transform.vec", "transform.quad", "transform.rect",
	"invertibleTransform.identity", "invertibleTransform.translate", "invertibleTransform.scale", "invertibleTransform.scaleAbout",
	"invertibleTransform.rotate", "invertibleTransform.rotateAbout", "invertibleTransform.shearX", "invertibleTransform.shearY",
	"invertibleTransform.simplePerspectiveX", "invertibleTransform.simplePerspectiveY", "invertibleTransform.perspectiveX",
	"invertibleTransform.perspectiveY", "invertibleTransform.normalize", "invertibleTransform.compose", "invertibleTransform.composeBefore",
	"invertibleTransform.vec", "invertibleTransform.inverseVec", "invertibleTransform.quad", "invertibleTransform.inverseQuad", "invertibleTransform.rect", "invertibleTransform.inverseRect",
	"transform.perspectiveApproach",
)

var resourceOperations = operationSet(
	"archetype.spawn", "archetype.id", "archetype.key", "entity.currentRef", "entityRef.get", "entityRef.getUnchecked", "entityRef.key", "entityRef.as", "entityRef.matches", "entityRef.getAs", "touch.values", "touch.items", "sprite.draw", "sprite.exists", "sprite.drawCurvedB", "sprite.drawCurvedT",
	"sprite.drawCurvedL", "sprite.drawCurvedR", "sprite.drawCurvedBT", "sprite.drawCurvedLR", "judgment.judge",
	"clip.play", "clip.playScheduled", "clip.playLooped", "clip.playLoopedScheduled", "loop.stop",
	"loop.stopScheduled", "particle.spawn", "particle.move", "particle.destroy", "instruction.show", "instruction.clear",
	"stream.set", "stream.has", "stream.previousKey", "stream.nextKey", "stream.get",
	"stream.previousKeyOrDefault", "stream.nextKeyOrDefault", "stream.hasPreviousKey", "stream.hasNextKey",
	"stream.previousKeyInclusive", "stream.nextKeyInclusive", "stream.getPrevious", "stream.getNext",
	"stream.getPreviousInclusive", "stream.getNextInclusive", "stream.itemsFrom", "stream.itemsFromDescending",
	"stream.itemsSincePreviousFrame", "stream.keysFrom", "stream.keysFromDescending", "stream.keysSincePreviousFrame",
	"stream.valuesFrom", "stream.valuesFromDescending", "stream.valuesSincePreviousFrame", "streamData.set", "streamData.get",
)

var containerOperations = operationSet(
	"varArray.new", "varArray.len", "varArray.capacity", "varArray.isFull", "varArray.get", "varArray.getUnchecked", "varArray.set", "varArray.setUnchecked",
	"varArray.append", "varArray.appendUnchecked", "varArray.pop", "varArray.clear", "varArray.contains", "varArray.insert",
	"varArray.removeAt", "varArray.remove", "varArray.index", "varArray.lastIndex", "varArray.count", "varArray.swap", "varArray.swapUnchecked", "varArray.reverse", "varArray.shuffle", "varArray.sortFunc", "varArray.extend",
	"varArray.indexMinFunc", "varArray.indexMaxFunc", "varArray.minFunc", "varArray.maxFunc", "varArray.values", "varArray.valuesReversed", "varArray.items", "entities.sortLinked", "entities.sortDoublyLinked",
	"arrayMap.new", "arrayMap.len", "arrayMap.capacity", "arrayMap.isFull", "arrayMap.get", "arrayMap.set", "arrayMap.delete",
	"arrayMap.contains", "arrayMap.clear",
	"arrayMap.getOK", "arrayMap.pop", "arrayMap.keys", "arrayMap.values", "arrayMap.items",
	"arraySet.new", "arraySet.len", "arraySet.capacity", "arraySet.isFull", "arraySet.add", "arraySet.remove", "arraySet.contains", "arraySet.clear", "arraySet.values",
)

func supportsRecipe(recipe catalog.Recipe) bool {
	var operations map[string]struct{}
	switch recipe.Kind {
	case catalog.RecipeAggregate:
		operations = aggregateOperations
	case catalog.RecipeResource:
		operations = resourceOperations
	case catalog.RecipeContainer:
		operations = containerOperations
	default:
		return true
	}
	_, ok := operations[recipe.Operation]
	return ok
}
