function applyDiff(parent, prop, index, ins, diff) {
    var value
    if (diff === null) {
        value = null
    } else if (typeof(diff) === "object") {
        if (diff._a !== undefined) {
            // Modify an array
            var arr = parent[prop]
            // Chop the array when necessary
            if (arr.length != diff._l) {
                arr.splice(diff._l, arr.length - diff._l)
            }
            var pos = arr.length
            var insertCount = 0
            for (let i = diff._a.length - 1; i >= 0; i--) {
                var e = diff._a[i]
                if (typeof(e) === "number") {
                    pos -= e
                } else if (e._d !== undefined) {
                    pos -= e._d
                    arr.splice(pos, e._d)
                } else if (e._i !== undefined) {
                    insertCount = e._i
                } else {
                    if (insertCount > 0) {
                        applyDiff(arr, undefined, pos, true, e)
                        insertCount--
                    } else {
                        pos--
                        applyDiff(arr, undefined, pos, false, e)
                    }
                }
            }
            return
        } else if (diff._id !== undefined) {
            // The value is an object literal
            value = diff
        } else {
            // Modify an object
            for (let key of Object.keys(diff)) {
                if (index === undefined) {
                    applyDiff(parent[prop], key, undefined, false, diff[key])
                } else {
                    applyDiff(parent[index], key, undefined, false, diff[key])
                }
            }
            return
        }
    } else if (Array.isArray(diff)) {
        // The value is an array literal
        value = diff
    } else {
        // The value is a primitive literal
        value = diff
    }

    // Set the property or list element
    if (index === undefined) {
        // Set property
        parent[prop] = diff
    } else {
        // Insert or replace an array element.
        // Use splice here to ensure vue.js compatibility
        if (ins) {
            parent.splice(index, 0, diff)
        } else {
            parent.splice(index, 1, diff)
        }
    }
}