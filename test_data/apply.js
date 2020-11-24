function applyDiff(parent, prop, index, diff) {
    var value
    if (diff === null) {
        value = null
    } else if (typeof(diff) === "object") {
        if (diff.$a !== undefined) {
            // Modify an array
            var arr = parent[prop]
            // Chop the array when necessary
            if (arr.length != diff.$l) {
                arr.splice(diff.$l, arr.length - diff.$l)
            }
            pos = arr.length
            for (let i = diff.$a.length - 1; i >= 0; i--) {
                var e = diff.$a[i]
                if (typeof(e) === "number") {
                    pos -= e
                } else if (e.$d !== undefined) {
                    pos -= e.$d
                    arr.splice(pos, e.$d)
                } else {
                    pos--
                    applyDiff(arr, undefined, pos, e)
                }
            }
            return
        } else if (diff.$m !== undefined) {
            // The value is an object literal
            value = diff
        } else {
            // Modify an object
            for (let key of Object.keys(diff)) {
                if (index === undefined) {
                    applyDiff(parent[prop], key, undefined, diff[key])
                } else {
                    applyDiff(parent[index], key, undefined, diff[key])
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
        // Set array element.
        // Use splice here to ensure vue.js compatibility
        parent.splice(index, 1, diff)
    }
}