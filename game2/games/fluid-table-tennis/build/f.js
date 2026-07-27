// Based on http://www.dgp.toronto.edu/people/stam/reality/Research/pdf/GDC03.pdf
/**
 ****************************************************
 * Copyright (c) 2009 Oliver Hunt <http://nerget.com>
 ****************************************************
 * Copyright (c) 2012 Anirudh Joshi <http://anirudhjoshi.com>
 **************************************************** 
 * Copyright (c) 2008, 2009, Memo Akten, www.memo.tv
 *** The Mega Super Awesome Visuals Company ***
 ****************************************************
 * All rights reserved. 
 * 
 * Permission is hereby granted, free of charge, to any person
 * obtaining a copy of this software and associated documentation
 * files (the "Software"), to deal in the Software without
 * restriction, including without limitation the rights to use,
 * copy, modify, merge, publish, distribute, sublicense, and/or sell
 * copies of the Software, and to permit persons to whom the
 * Software is furnished to do so, subject to the following
 * conditions:
 * 
 * The above copyright notice and this permission notice shall be
 * included in all copies or substantial portions of the Software.
 * 
 * THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND,
 * EXPRESS OR IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES
 * OF MERCHANTABILITY, FITNESS FOR A PARTICULAR PURPOSE AND
 * NONINFRINGEMENT. IN NO EVENT SHALL THE AUTHORS OR COPYRIGHT
 * HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY,
 * WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING
 * FROM, OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR
 * OTHER DEALINGS IN THE SOFTWARE.
 */

 // Check if we have access to contexts
if ( this.CanvasRenderingContext2D && !CanvasRenderingContext2D.createImageData ) {
    
    // Grabber helper function
    CanvasRenderingContext2D.prototype.createImageData = function ( w, h ) {
        
        return this.getImageData( 0, 0, w, h);
        
    }
    
}

function Fluid(canvas) {
    // Add fields x and s together over dt
    function addFields(x, s, dt) {

        for ( var i = 0; i < size; i++ ) {
            
            x[ i ] += dt * s[ i ];

        }

    } 

    // Fluid bounding function over a field for stability
    function set_bnd(b, x) {


        if ( b === 1 ) {

            for ( var i = 1; i <= width; i++ ) {

                x[ i ] =  x[ i + rowSize ];
                x[ i + ( height + 1 ) * rowSize] = x[ i + height * rowSize ];

            }

            for (var j = 1; i <= height; i++) {

                x[j * rowSize] = -x[1 + j * rowSize];
                x[(width + 1) + j * rowSize] = -x[width + j * rowSize];

            }

        } else if (b === 2) {
            for (var i = 1; i <= width; i++) {
                x[i] = -x[i + rowSize];
                x[i + (height + 1) * rowSize] = -x[i + height * rowSize];
            }

            for (var j = 1; j <= height; j++) {
                x[j * rowSize] =  x[1 + j * rowSize];
                x[(width + 1) + j * rowSize] =  x[width + j * rowSize];
            }
        } else {
            for (var i = 1; i <= width; i++) {
                x[i] =  x[i + rowSize];
                x[i + (height + 1) * rowSize] = x[i + height * rowSize];
            }

            for (var j = 1; j <= height; j++) {
                x[j * rowSize] =  x[1 + j * rowSize];
                x[(width + 1) + j * rowSize] =  x[width + j * rowSize];
            }
        }

        var maxEdge = (height + 1) * rowSize;

        x[ 0 ]                 = 0.5 * ( x[ 1 ] + x[ rowSize ] );
        x[ maxEdge ]           = 0.5 * (x[1 + maxEdge] + x[height * rowSize]);
        x[ ( width + 1 ) ]         = 0.5 * (x[width] + x[(width + 1) + rowSize]);
        x[ ( width + 1) + maxEdge ] = 0.5 * (x[width + maxEdge] + x[(width + 1) + height * rowSize]);

    }

    // This combines neighbour velocities onto selected cell
    function lin_solve(b, x, x0, a, c) {

        if (a === 0 && c === 1) {

            for (var j=1 ; j<=height; j++) {

                var currentRow = j * rowSize;

                ++currentRow;

                for ( var i = 0; i < width; i++ ) {

                    x[currentRow] = x0[currentRow];
                    ++currentRow;

                }

            }

            set_bnd(b, x);

        } else {

            var invC = 1 / c;

            for (var k=0 ; k<iterations; k++) {

                for (var j=1 ; j<=height; j++) {

                    var lastRow = (j - 1) * rowSize;
                    var currentRow = j * rowSize;
                    var nextRow = (j + 1) * rowSize;
                    var lastX = x[currentRow];

                    ++currentRow;

                    for (var i=1; i<=width; i++)

                        lastX = x[currentRow] = (x0[currentRow] + a*(lastX+x[++currentRow]+x[++lastRow]+x[++nextRow])) * invC;

                }

                set_bnd(b, x);

            }

        }

    }

    var fadeSpeed = 0.01;
    var holdAmount = 1 - fadeSpeed;

    // Fades out velocities/densities to stop full stability
    // MSAFluidSolver2d.java
    function fade( x ) {

        for (var i = 0; i < size; i++) {

            // fade out
            x[i] *= holdAmount;

        }

        return;
    }    

    // Iterates over the entire array - diffusing dye density
    function diffuse(b, x, x0, dt) {

        var a = 0;
        lin_solve(b, x, x0, a, 1 + 4*a);

    }
    
    function lin_solve2(x, x0, y, y0, a, c)
    {
        if (a === 0 && c === 1) {
            for (var j=1 ; j <= height; j++) {
                var currentRow = j * rowSize;
                ++currentRow;
                for (var i = 0; i < width; i++) {
                    x[currentRow] = x0[currentRow];
                    y[currentRow] = y0[currentRow];
                    ++currentRow;
                }
            }
            set_bnd(1, x);
            set_bnd(2, y);
        } else {
            var invC = 1/c;
            for (var k=0 ; k<iterations; k++) {
                for (var j=1 ; j <= height; j++) {
                    var lastRow = (j - 1) * rowSize;
                    var currentRow = j * rowSize;
                    var nextRow = (j + 1) * rowSize;
                    var lastX = x[currentRow];
                    var lastY = y[currentRow];
                    ++currentRow;
                    for (var i = 1; i <= width; i++) {
                        lastX = x[currentRow] = (x0[currentRow] + a * (lastX + x[currentRow] + x[lastRow] + x[nextRow])) * invC;
                        lastY = y[currentRow] = (y0[currentRow] + a * (lastY + y[++currentRow] + y[++lastRow] + y[++nextRow])) * invC;
                    }
                }
                set_bnd(1, x);
                set_bnd(2, y);
            }
        }
    }
    
    function diffuse2(x, x0, y, y0, dt)
    {
        var a = 0;
        lin_solve2(x, x0, y, y0, a, 1 + 4 * a);
    }
    
    function advect(b, d, d0, u, v, dt)
    {
        var Wdt0 = dt * width;
        var Hdt0 = dt * height;
        var Wp5 = width + 0.5;
        var Hp5 = height + 0.5;
        for (var j = 1; j<= height; j++) {
            var pos = j * rowSize;
            for (var i = 1; i <= width; i++) {
                var x = i - Wdt0 * u[++pos]; 
                var y = j - Hdt0 * v[pos];
                if (x < 0.5)
                    x = 0.5;
                else if (x > Wp5)
                    x = Wp5;
                var i0 = x | 0;
                var i1 = i0 + 1;
                if (y < 0.5)
                    y = 0.5;
                else if (y > Hp5)
                    y = Hp5;
                var j0 = y | 0;
                var j1 = j0 + 1;
                var s1 = x - i0;
                var s0 = 1 - s1;
                var t1 = y - j0;
                var t0 = 1 - t1;
                var row1 = j0 * rowSize;
                var row2 = j1 * rowSize;
                d[pos] = s0 * (t0 * d0[i0 + row1] + t1 * d0[i0 + row2]) + s1 * (t0 * d0[i1 + row1] + t1 * d0[i1 + row2]);
            }
        }
        set_bnd(b, d);
    }
    
    function project(u, v, p, div)
    {
        var h = -0.5 / Math.sqrt(width * height);
        for (var j = 1 ; j <= height; j++ ) {
            var row = j * rowSize;
            var previousRow = (j - 1) * rowSize;
            var prevValue = row - 1;
            var currentRow = row;
            var nextValue = row + 1;
            var nextRow = (j + 1) * rowSize;
            for (var i = 1; i <= width; i++ ) {
                div[++currentRow] = h * (u[++nextValue] - u[++prevValue] + v[++nextRow] - v[++previousRow]);
                p[currentRow] = 0;
            }
        }
        set_bnd(0, div);
        set_bnd(0, p);
        
        lin_solve(0, p, div, 1, 4 );
        var wScale = 0.5 * width;
        var hScale = 0.5 * height;
        for (var j = 1; j<= height; j++ ) {
            var prevPos = j * rowSize - 1;
            var currentPos = j * rowSize;
            var nextPos = j * rowSize + 1;
            var prevRow = (j - 1) * rowSize;
            var currentRow = j * rowSize;
            var nextRow = (j + 1) * rowSize;

            for (var i = 1; i<= width; i++) {
                u[++currentPos] -= wScale * (p[++nextPos] - p[++prevPos]);
                v[currentPos]   -= hScale * (p[++nextRow] - p[++prevRow]);
            }
        }
        set_bnd(1, u);
        set_bnd(2, v);
    }
    
    // Move forward in density
    function dens_step(r_prev, g_prev, bl_prev, u_prev, v_prev, r, g, bl, u, v, dt ) {

        // Stop filling stability
        fade( r );
        fade( g );
        fade( bl );

        // fade( u );
        // fade( v );        

        // Combine old and new fields into the new field
        addFields( r, r_prev, dt);
        addFields( g, g_prev, dt);
        addFields( bl, bl_prev, dt);

        // Diffuse over old and new new field
        diffuse(0, r_prev, r, dt );
        diffuse(0, g_prev, g, dt );
        diffuse(0, bl_prev, bl, dt );

        // Combine vectors into a forward vector model
        advect(0, r, r_prev, u, v, dt );
        advect(0, g, g_prev, u, v, dt );
        advect(0, bl, bl_prev, u, v, dt );

    }
    
    // Move vector fields (u,v) forward over dt
    function vel_step(u, v, u0, v0, dt) {

        addFields(u, u0, dt );
        addFields(v, v0, dt );
        
        var temp = u0; u0 = u; u = temp;
        var temp = v0; v0 = v; v = temp;
        
        diffuse2(u,u0,v,v0, dt);
        project(u, v, u0, v0);
        
        var temp = u0; u0 = u; u = temp; 
        var temp = v0; v0 = v; v = temp;
        
        advect(1, u, u0, u0, v0, dt);
        advect(2, v, v0, u0, v0, dt);
        
        project(u, v, u0, v0 );

    }

    var uiCallback = function( r, g, bl, u, v ) {};

    function Field(r, g, bl, u, v) {

        // Just exposing the fields here rather than using accessors is a measurable win during display (maybe 5%)
        // but makes the code ugly.

        this.setDensityRGB = function(x, y, d) {

             r[(x + 1) + (y + 1) * rowSize] = d[0];
             g[(x + 1) + (y + 1) * rowSize] = d[1];
             bl[(x + 1) + (y + 1) * rowSize] = d[2];

             return;

        }

        this.getDensityRGB = function(x, y) {

             var r_dens = r[(x + 1) + (y + 1) * rowSize];
             var g_dens = g[(x + 1) + (y + 1) * rowSize];
             var bl_dens = bl[(x + 1) + (y + 1) * rowSize];

             return [ r_dens, g_dens, bl_dens ];

        }        

        this.setVelocity = function(x, y, xv, yv) {

             u[(x + 1) + (y + 1) * rowSize] = xv;
             v[(x + 1) + (y + 1) * rowSize] = yv;

             return;

        }

        //MSAFluidSolver2d.java
        this.setVelocityInterp = function( x, y, vx, vy ) {

            var colSize = rowSize;

            rI = x + 2;
            rJ = y + 2;

            i1 = (x + 2);
            i2 = (rI - i1 < 0) ? (x + 3) : (x + 1);

            j1 = (y + 2);
            j2 = (rJ - j1 < 0) ? (y  + 3) : (y + 1);
            
            diffx = (1-(rI-i1));
            diffy = (1-(rJ-j1));
            
            vx1 = vx * diffx*diffy;
            vy1 = vy * diffy*diffx;
            
            vx2 = vx * (1-diffx)*diffy;
            vy2 = vy * diffy*(1-diffx);
            
            vx3 = vx * diffx*(1-diffy);
            vy3 = vy * (1-diffy)*diffx;
            
            vx4 = vx * (1-diffx)*(1-diffy);
            vy4 = vy * (1-diffy)*(1-diffx);
            
            if(i1<2 || i1>rowSize-1 || j1<2 || j1>colSize-1) return;

            this.setVelocity(i1, j1, vx1, vy1);
            this.setVelocity(i2, j1, vx2, vy2);
            this.setVelocity(i1, j2, vx3, vy3);
            this.setVelocity(i2, j2, vx4, vy4);

            return;

        }         


        this.getXVelocity = function(x, y) {

             var x_vel = u[(x + 1) + (y + 1) * rowSize];

             return x_vel;

        }
        
        this.getYVelocity = function(x, y) {

             var y_vel = v[(x + 1) + (y + 1) * rowSize];

             return y_vel;

        }

        this.width = function() { return width; }
        this.height = function() { return height; }

    }

    function queryUI( r, g, bl, u, v ) {

        for ( var i = 0; i < size; i++ )

            r[ i ] = g[i] = bl[i] = 0.0;

        // u[ i ] = v[ i ] = - figure out better way!

        uiCallback( new Field( r, g, bl, u, v ) );

    }

    // Push simulation forward one step
    this.update = function () {

        queryUI(r_prev, g_prev, bl_prev, u_prev, v_prev);

        // Move vector fields forward
        vel_step(u, v, u_prev, v_prev, dt);

        // Move dye intensity forward
        // dens_step(dens, dens_prev, u, v, dt);
        if ( u_prev )
            dens_step(r_prev, g_prev, bl_prev, u_prev, v_prev, r, g, bl, u, v, dt );

        // Display/Return new density and vector fields
        displayFunc( new Field(r, g, bl, u, v) );

    }

    this.setDisplayFunction = function( func ) {

        displayFunc = func;

    }
    
    // More iterations = much slower simulation (10 is good default)
    this.iterations = function() { return iterations; }

    // Iteration setter and capper
    this.setIterations = function( iters ) {

        if ( iters > 0 && iters <= 100 )

           iterations = iters;

    }

    this.setUICallback = function( callback ) {
        
        uiCallback = callback;

    }

    var iterations = 10;

    var visc = 0.5;
    var dt = 0.1;

    var r;
    var r_prev;

    var g;
    var g_prev;

    var bl;
    var bl_prev;    

    var u;
    var u_prev;

    var v;
    var v_prev;

    var width;
    var height;

    var rowSize;
    var size;

    var displayFunc;

    function reset() {

        rowSize = width + 2;
        size = (width+2)*(height+2);

        r = new Array(size);
        r_prev = new Array(size);        

        g = new Array(size);
        g_prev = new Array(size);        

        bl = new Array(size);
        bl_prev = new Array(size);                

        u = new Array(size);
        u_prev = new Array(size);

        v = new Array(size);
        v_prev = new Array(size);

        for (var i = 0; i < size; i++) {

            u_prev[i] = v_prev[i] = u[i] = v[i] = 0;
            r[i] = g[i] = bl[i] = r_prev[i] = g_prev[i] = bl_prev[i] = 0;

        }

    }

    this.reset = reset;

    this.fieldRes = 96;

    // Resolution bounder and resetter
    this.setResolution = function ( hRes, wRes ) {

        var res = wRes * hRes;

        this.fieldRes = hRes;

        if (res > 0 && res < 1000000 && (wRes != width || hRes != height)) {

            width = wRes;
            height = hRes;

            reset();

            return true;

        }
        
        return false;
    }


    // Store the alpha blending data in a unsigned array
    var buffer;
    var bufferData;
    var clampData = false;
    
    var canvas = document.getElementById("canvas");;
    
    // First run to generate alpha blending array
    function prepareBuffer(field) {
        
        // Check bounds/existance between blending data and simulation field
        if ( buffer && buffer.width == field.width() && buffer.height == field.height() )
        
            return;
        
        // Else create buffer array    
        buffer = document.createElement("canvas");
        buffer.width = field.width();
        buffer.height = field.height();
        
        var context = buffer.getContext("2d");
        
        try {
            
            // Try to fill up using helper function
            bufferData = context.createImageData( field.width(), field.height() );
            
        } catch(e) {
            
            return null;
            
        }
        
        // Return for non-existant canvas
        if (!bufferData)
        
            return null;
            
        // Generate over square buffer array (r,b,g,a)
        var max = field.width() * field.height() * 4;

        for ( var i = 3; i < max; i += 4 )
            
            // Set all alpha values to maximium opacity
            bufferData.data[i] = 255;
            
        bufferData.data[0] = 256;
        
        if (bufferData.data[0] > 255)
        
            clampData = true;
            
        bufferData.data[0] = 0;
        


    }

    function displayDensity(field) {
        
        var context = canvas.getContext("2d");
        var width = field.width();
        var height = field.height();
        
        // Continously buffer data to reduce computation overhead
        prepareBuffer(field);        

        if ( pong.display ){
        
            if ( pong.ball.x < width && pong.ball.x > 0 && pong.ball.y > 0 && pong.ball.y < height ){
        
                pong.ball.vy += field.getYVelocity(Math.round( pong.ball.x ), Math.round( pong.ball.y ) ) / 4;
                pong.ball.vx += field.getXVelocity(Math.round( pong.ball.x ), Math.round( pong.ball.y ) ) / 6;
                
            }
            
        }

        if (bufferData) {
            
            // Decouple from pixels to reduce overhead
            var data = bufferData.data;

            var dlength = data.length;
            var j = -3;
            
            if ( clampData ) {
                
                for ( var x = 0; x < width; x++ ) {
                    
                    for ( var y = 0; y < height; y++ ) {
                        
                        var d = field.getDensity(x, y) * 255 / 5;
                        
                        d = d | 0;
                        
                        if ( d > 255 )
                        
                            d = 255;
                            
                        data[ 4 * ( y * height + x ) + 1] = d;
                        
                    }
                    
                }
                
            } else {
                // console.log( field.getDensity(1, 1) );

                for ( var x = 0; x < width; x++ ) {


                    for ( var y = 0; y < height; y++ ) {

                        var index = 4 * (y * height +  x);                        
                        var RGB = field.getDensityRGB(x, y);     

                        data[ index + 0] = Math.round( RGB[0] * 255 / 5 );
                        data[ index + 1] = Math.round( RGB[1] * 255 / 5 );
                        data[ index + 2] = Math.round( RGB[2] * 255 / 5 );

                    }
                        
                }
                
            }

            context.putImageData(bufferData, 0, 0);
            
        } else {
            
            for ( var x = 0; x < width; x++ ) {
                
                for ( var y = 0; y < height; y++ ) {
                    
                    var d = field.getDensity(x, y) / 5;
                    
                    context.setFillColor(0, d, 0, 1);
                    context.fillRect(x, y, 1, 1);
                    
                }
                
            }
            
        }
        
    }
    
    function displayVelocity( field ) {
        
        var context = canvas.getContext("2d");
        
        context.save();
        context.lineWidth = 1;
        
        var wScale = canvas.width / field.width();
        var hScale = canvas.height / field.height();
        
        context.fillStyle="black";
        context.fillRect(0, 0, canvas.width, canvas.height);
        context.strokeStyle = "rgb(0,255,0)";
        
        var vectorScale = 10;
        
        context.beginPath();
        
        for (var x = 0; x < field.width(); x++) {
            
            for (var y = 0; y < field.height(); y++) {
                
                context.moveTo(x * wScale + 0.5 * wScale, y * hScale + 0.5 * hScale);
                context.lineTo((x + 0.5 + vectorScale * field.getXVelocity(x, y)) * wScale, 
                               (y + 0.5 + vectorScale * field.getYVelocity(x, y)) * hScale);
                               
            }
            
        }
        
        context.stroke();
        context.restore();
        
    }
    
    this.toggleDisplayFunction = function( canvas, showVectors ) {

        if (showVectors) {
            
            showVectors = false;
            canvas.width = displaySize;
            canvas.height = displaySize;
            
            return displayVelocity;
            
        }
        
        showVectors = true;
        
        canvas.width = this.fieldRes;
        canvas.height = this.fieldRes;
        
        return displayDensity;
        
    }

}/**
 ****************************************************
 * Copyright (c) 2012 Anirudh Joshi <http://anirudhjoshi.com>
 **************************************************** 
 * All rights reserved. 
 * 
 * Permission is hereby granted, free of charge, to any person
 * obtaining a copy of this software and associated documentation
 * files (the "Software"), to deal in the Software without
 * restriction, including without limitation the rights to use,
 * copy, modify, merge, publish, distribute, sublicense, and/or sell
 * copies of the Software, and to permit persons to whom the
 * Software is furnished to do so, subject to the following
 * conditions:
 * 
 * The above copyright notice and this permission notice shall be
 * included in all copies or substantial portions of the Software.
 * 
 * THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND,
 * EXPRESS OR IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES
 * OF MERCHANTABILITY, FITNESS FOR A PARTICULAR PURPOSE AND
 * NONINFRINGEMENT. IN NO EVENT SHALL THE AUTHORS OR COPYRIGHT
 * HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY,
 * WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING
 * FROM, OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR
 * OTHER DEALINGS IN THE SOFTWARE.
 */

// better control method?
function Pong(canvas) {

	"use strict";

	this.canvas = canvas;
	this.ctx = this.canvas.getContext('2d');

	this.theta = 0;
	this.speed_increase = 0.7;
	this.speed = 1;

	this.display = true;

	var player = function () {

		this.life = 5;
		this.push = true;
		this.suck = false;
		this.stream = [ 0, 0, 0];
		this.multiplayer = false;
		this.x  = 0;
		this.y  = 0;
		this.width  = 0;
		this.height  = 0;
		this.color  = "red";
		this.vx  = 0;
		this.vy  = 0;
		this.ax  = 0;
		this.ay  = 0;
		this.xo  = 0;
		this.yo  = 0;
		this.out  = false;
		this.radius = 0;
		this.speed = 1

	};

	this.ball = new player();
	this.ai = new player();
	this.player = new player();

	this.updatePlayer = function () {

		this.player.vy += this.player.ay;					

		if (this.keyMap.up.on) {
			this.player.ay = -this.speed_increase;
			if (this.player.vy < -this.speed) {
				this.player.vy = -this.speed;
			}
		}
		if (this.keyMap.right.on) {
			this.player.push = true;
		} else {
			this.player.push = false;
		}
		if (this.keyMap.left.on) {
			this.player.suck = true;
		} else {
			this.player.suck = false;
		}	
		if (this.keyMap.down.on) {
			this.player.ay = this.speed_increase;
			
			if (this.player.vy > this.speed) {
				this.player.vy = this.speed;
				
			}

		}
		
		if ( ( !(this.keyMap.down.on) && !(this.keyMap.up.on) ) || (this.keyMap.down.on && this.keyMap.up.on) ) {
			
			this.player.ay = 0;
			this.player.vy = 0;
		
		}
		
		if ( ( this.player.y < 0 && this.player.vy < 0 ) || ( this.player.y + this.player.height > this.ctx.canvas.height && this.player.vy > 0 ) ) {
			
			this.player.ay = 0;
			this.player.vy = 0;
			
		}

		
		this.player.y += this.player.vy;    

	};

	this.updateAi = function () {

		var real_y_pos = 0;

		if ( this.ai.multiplayer ) {

			this.ai.vy += this.ai.ay;     

			if ( this.keyMap.up2.on ) {
			
				this.ai.ay = -this.speed_increase;
				
				if ( this.ai.vy < -this.speed ) {
					
					this.ai.vy = -this.speed;
					
				}
				
			}                    
			
			if ( this.keyMap.left2.on ) {
			
				this.ai.push = true;

				
			} else {
				
				this.ai.push = false;
				
			}
			
			if ( this.keyMap.right2.on ) {
			
				this.ai.suck = true;

				
			} else {
				
				this.ai.suck = false;
				
			}	
			
			if ( this.keyMap.down2.on ) {
			
				this.ai.ay = this.speed_increase;
				
				if ( this.ai.vy > this.speed ) {
					
					this.ai.vy = this.speed;
					
				}

			}
			
			if ( ( !(this.keyMap.down2.on) && !(this.keyMap.up2.on) ) || (this.keyMap.down2.on && this.keyMap.up2.on) ) {
				
				this.ai.ay = 0;
				this.ai.vy = 0;
			
			}
			
			if ( ( this.ai.y < 0 && this.ai.vy < 0 ) || ( this.ai.y + this.ai.height > this.ctx.canvas.height && this.ai.vy > 0 ) ) {
				
				this.ai.ay = 0;
				this.ai.vy = 0;
				
			}
			
			this.ai.y += this.ai.vy;    


		} else {

			// calculate the middle of the paddle 
			real_y_pos = this.ai.y + (this.ai.height / 2); 

			/* If the this.ball is moving in opposite direction to the paddle and is no danger for computer's goal move paddle back to the middle y - position*/ 
			if ( this.ball.vx < 0 ) {

				// if the paddle's position is over the middle y - position 
				if ( real_y_pos < this.ctx.canvas.height / 2 - this.ctx.canvas.height / 10 ) {
				
					this.ai.y  += this.speed; 
					
				} 

				// Paddle is under the middle y - position 
				else if (   real_y_pos > this.ctx.canvas.height / 2 + this.ctx.canvas.height / 10  ) {
				
					this.ai.y  -= this.speed; 
					
				}
				
			} 
			// this.ball is moving towards paddle 
			else if ( this.ball.vx > 0 ) {

				// As long as this.ball's y - position and paddle's y - position are different 
				if (  Math.abs(this.ball.y - real_y_pos ) > 2 ) {
				
					// If this.ball's position smaller than paddle's, move up 
					if (this.ball.y < real_y_pos) {
					
						this.ai.y -= this.speed/1.2; 
						
					} 
					
					// If this.ball's position greater than padle's, move down 
					else if ( this.ball.y > real_y_pos ) {
					
						this.ai.y  += this.speed/1.2; 
					 
					}
				
				}

			}
		}

	};

	this.updateBall = function (){

		if ( ( Math.abs( this.ball.x - this.player.x ) < Math.abs( this.ball.vx ) && this.player.y < this.ball.y + 0.1 * this.player.height && this.ball.y < this.player.y + 1.1 * this.player.height ) ) {
			this.theta = ((this.player.y + this.player.height/2) - this.ball.y ) / ( this.player.height  /  2 );
			this.ball.vx = this.ball.speed * Math.cos(this.theta);
			this.ball.vy = -this.ball.speed * Math.sin(this.theta);

		}
		
		if ( ( Math.abs(this.ball.x - this.ai.x) < Math.abs( this.ball.vx ) && this.ai.y < this.ball.y + this.ai.height && this.ball.y < this.ai.y + this.ai.height ) ) {

			this.theta = ((this.ai.y + this.ai.height/2) - this.ball.y ) / ( this.ai.height  /  2 );
			this.ball.vx = -this.ball.speed * Math.cos(this.theta);
			this.ball.vy = -this.ball.speed * Math.sin(this.theta);
		}

		// y
		if ( ( this.ball.y + this.ball.vy < 0 && this.ball.vy < 0 ) || ( this.ball.y + this.ball.radius + this.ball.vy > this.ctx.canvas.height && this.ball.vy > 0 ) ) {
			
			this.ball.vy = -this.ball.vy;

		}

		// x
		// + this.ball.radius
		if ( ( this.ball.x < 0 && this.ball.vx < 0 ) || ( this.ball.x > this.ctx.canvas.width && this.ball.vx > 0 ) ) {

			if ( this.ball.x < 0) {

				this.player.life -= 1;

			}

			if ( this.ball.x > this.ctx.canvas.width){

				this.ai.life -= 1;

			}

			this.ball.xo = this.ball.x;
			this.ball.yo = this.ball.y;

			this.ball.out = true;

			this.ball.x = ( this.ctx.canvas.width - this.ball.radius ) / 2;
			this.ball.y = this.ctx.canvas.height / 2;
			
			this.theta = Math.random() * 2*Math.PI;

			if (this.theta > Math.PI/4 && this.theta < 3* Math.PI/4) {

				this.theta = Math.round(Math.random()) === 1 ? Math.PI/4 : 3 * Math.PI/4;

			}

			if (this.theta > Math.PI + Math.PI/4 && this.theta < 3* Math.PI/4 + Math.PI) {

				this.theta = Math.round(Math.random()) === 1 ? Math.PI/4 + Math.PI : 3 * Math.PI/4 + Math.PI;

			}

			this.ball.vx = this.ball.speed * Math.cos(this.theta);
			this.ball.vy = this.ball.speed * Math.sin(this.theta);
			
			if ( Math.round( Math.random() ) === 1 ) {
			
				this.ball.vy = -this.ball.vy;
			
			}
			
		}		

		// this.ball.vx += this.ball.ax;
		this.ball.x += this.ball.vx;

		// this.ball.vy += this.ball.ay;   
		this.ball.y += this.ball.vy; 

	};

	this.distance = function ( player1, player2 ) {

		return Math.sqrt( Math.pow(player2.x - player1.x, 2) + Math.pow( player2.y - player1.y, 2) );

	};

	this.update = function(){

		if ( this.display ){

			this.updatePlayer();
			this.updateAi();
			this.updateBall();			

		} else {

			this.player.push = true;
			this.ai.push = true;

		}

	};

	this.clear = function () {

		// attribute - stack overflow answer
		// Store the current transformation matrix
		this.ctx.save();

		// Use the identity matrix while clearing the canvas
		this.ctx.setTransform(1, 0, 0, 1, 0, 0);
		this.ctx.clearRect(0, 0, this.ctx.canvas.width, this.ctx.canvas.height);

		// Restore the transform
		this.ctx.restore();


	};

	this.drawRectangle = function( x, y, width, height, color ) {

		if (color instanceof Array) {

			this.ctx.fillStyle = "rgb(" + Math.floor( color[0] ) + "," + Math.floor( color[1] ) + "," + Math.floor( color[2] ) + ")";

		} else {

			this.ctx.fillStyle = color;

		}

		this.ctx.fillRect( x, y, width, height );
		
	};

	this.drawPlayer = function( player ) {

		this.drawRectangle( player.x , player.y, player.width, player.height, player.color ) ;                    

	};

	this.drawBall = function ( ball ) {

		this.ctx.beginPath();
		this.ctx.lineWidth = 0.5;
		this.ctx.fillStyle = "black";
		this.ctx.strokeStyle = "white";        
		this.ctx.arc(ball.x, ball.y, ball.radius, 0, 2 * Math.PI, false);
		this.ctx.fill();
		this.ctx.stroke();
	
	};


	this.render = function() {

		if ( this.display ){

			this.drawPlayer( this.player );
			this.drawPlayer( this.ai );                    
			this.drawBall( this.ball );

		}

	};

	this.loop = function() {

		this.update();
		this.render();
		
	};

	this.init = function() {

		var pong_width = this.ctx.canvas.width / 75,
			pong_height = this.ctx.canvas.height / 6,
			screen_width = this.ctx.canvas.width,
			screen_height= this.ctx.canvas.height;
		
		this.player.life = 5;
		this.ai.life = 5;

		this.ai.width = pong_width;
		this.ai.height = pong_height;
		
		this.ai.x =  screen_width - this.ai.width;
		this.ai.y = screen_height / 2;
		
		this.player.width = pong_width;
		this.player.height = pong_height;               
		
		this.player.y = screen_height / 2;
		
		this.ball.radius = pong_width*2;
			
		this.theta = Math.PI;
		
		if ( this.theta > Math.PI/4 && this.theta < 3* Math.PI/4 ) {
			
			this.theta = Math.round(Math.random()) === 1 ? Math.PI/4 : 3 * Math.PI/4;
			
		}
		
		if( this.theta > Math.PI + Math.PI/4 && this.theta < 3* Math.PI/4 + Math.PI ) {
			
			this.theta = Math.round(Math.random()) === 1 ? Math.PI/4 + Math.PI : 3 * Math.PI/4 + Math.PI;
			
		}
		
		this.ball.vx = this.ball.speed * Math.cos(this.theta);
		this.ball.vy = this.ball.speed * Math.sin(this.theta);
		
		this.ball.x = this.ctx.canvas.width / 2;
		this.ball.y = this.ctx.canvas.height / 2;
		
	};

	this.keyMap = {

		"left": { "code" : 65, "on": false },
		"up": { "code" : 87, "on": false },
		"right": { "code" : 68, "on": false },
		"down": { "code" : 83, "on": false },
		"left2": { "code" : 74, "on": false },
		"up2": { "code" : 73, "on": false },
		"right2": { "code" : 76, "on": false },
		"down2": { "code" : 75, "on": false }

	};

}/**
 ****************************************************
 * Copyright (c) 2012 Anirudh Joshi <http://anirudhjoshi.com>
 **************************************************** 
 * All rights reserved. 
 * 
 * Permission is hereby granted, free of charge, to any person
 * obtaining a copy of this software and associated documentation
 * files (the "Software"), to deal in the Software without
 * restriction, including without limitation the rights to use,
 * copy, modify, merge, publish, distribute, sublicense, and/or sell
 * copies of the Software, and to permit persons to whom the
 * Software is furnished to do so, subject to the following
 * conditions:
 * 
 * The above copyright notice and this permission notice shall be
 * included in all copies or substantial portions of the Software.
 * 
 * THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND,
 * EXPRESS OR IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES
 * OF MERCHANTABILITY, FITNESS FOR A PARTICULAR PURPOSE AND
 * NONINFRINGEMENT. IN NO EVENT SHALL THE AUTHORS OR COPYRIGHT
 * HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY,
 * WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING
 * FROM, OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR
 * OTHER DEALINGS IN THE SOFTWARE.
 */

function Colors(){

	this.distanceRotators = [ 0, 201, 401 ];
	this.colors = [[0,0,0],[0,0,0],[0,0,0]];

	this.white = [0.9642, 1, 0.8249 ];	
	this.L = 75;

	function cielabToRGB( L, ab, white ) {

		x = white[0] * inverseCielab( ( 1 / 116 ) * ( L + 16 ) + ( 1 / 500 ) * ab[0] ) * 255;
		y = white[1] * inverseCielab( ( 1 / 116 ) * ( L + 16 ) ) * 255;
		z = white[2] * inverseCielab( ( 1 / 116 ) * ( L + 16 ) + ( 1 / 200 ) * ab[1] ) * 255;

		return [ x, y, z];
	}

	function inverseCielab( t ) {

		if ( t > ( 6 / 29 ) ){

			return Math.pow( t, 3);

		} else {

			return 3 * Math.pow( 6 / 29, 2 )* ( t - 4 / 29);

		}


	}

	function rotator( a ) {

		if ( a >= 0 && a <= 200 ){

			return [ 200 - (100 + a), 100 ];

		} else if ( a > 200 && a <= 400 ) {

			return [ -100 + (a - 200), 100 - (a - 200) ];

		} else if ( a > 400 && a <= 600 ) {

			return [ 100, -100 + (a - 400) ]

		}

	}	

	this.rotate = function(){

		for ( var i = 0; i < 3; i++ ){

			this.colors[i] = cielabToRGB( this.L, rotator( this.distanceRotators[i]), this.white );

			this.distanceRotators[i]++;

			if ( this.distanceRotators[i] > 600 ) {

				this.distanceRotators[i] = 0;

			}

		}		

	}

}// If you are viewing this code - don't judge me too harshly :D
// Still needs a good clean up and encapsulation - just aimed to get it working
// as quickly as I could

// Global settings
var FPS = 60;
var running = false;
var canvas = document.getElementById("canvas");
var ctx = canvas.getContext("2d");

// Globals
var field;
var pong;
var colors;
var counter;

function toggleMultiplayer() {	

	if ( pong.ai.multiplayer ){

		pong.ai.multiplayer = false;
		pong.ai.push = true;
		document.getElementById("multiplayer").innerHTML = "Begin Multiplayer"

	} else {

		pong.ai.multiplayer = true;

		document.getElementById("multiplayer").innerHTML = "Begin Single Player"

	}		

	restart();

}

function restart() {

	suck_counter_1 = 100;
	suck_counter_2 = 100;

	prev_player_life = 5;
	prev_ai_life = 5;

	counter.coul_incr = 0;

 	field.reset();

	pong.display = false;
	pong.init();

 	pong.clear();

	counter.run_coul = true;

}

var ball_counter = 0;
var suck_counter_1 = 100;
var suck_counter_2 = 100;

var fps = 0;

var suck_on = false;
var ball_caught = false;

var suck_on2 = false;
var ball_caught2 = false;

var prev_player_life = 5;
var prev_ai_life = 5;

var ai_switch = "AI";
var player_switch = "HUMAN";

var not = document.getElementById('notifications');

function notify(){

	if ( pong.ai.multiplayer ){

		ai_switch = "PLAYER 2";
		player_switch = "PLAYER 1";

	} else {

		ai_switch = "AI";
		player_switch = "HUMAN";

	}

	if ( pong.player.life < prev_player_life ){

		not.innerHTML = ai_switch + " SCORES " + (5 - pong.player.life) + "!";
		prev_player_life = pong.player.life;

	}

	if ( pong.ai.life < prev_ai_life ){

		not.innerHTML = player_switch + " SCORES " + (5 - pong.ai.life) + "!";
		prev_ai_life = pong.ai.life;
		
	}	

	if ( pong.player.life == 0 ){

		not.innerHTML = ai_switch + " WINS!"
		restart();

	}

	if ( pong.ai.life == 0 ){

		not.innerHTML = player_switch + " WINS!"

		restart();

	}

}

function push( player, field, velocity ){

	if (player.push) {

		field.setVelocity( Math.floor( player.x + player.width / 2 ), Math.floor( player.y + player.height / 2 ), velocity, 0 );	
		field.setDensityRGB( Math.floor( player.x + player.width / 2  ) , Math.floor( player.y + player.height / 2 ), player.color);

	}

}

function explode(field){

	if ( pong.ball.out ){

		if ( pong.ball.xo > canvas.width / 2 ) {
			var x = canvas.width-1;
			var mult = -1;

	} else {

		var mult = 1;
		var x = 0;
	}

	field.setDensityRGB( x, Math.floor( pong.ball.yo + pong.ball.radius / 2 ), pong.ball.color );				
	field.setVelocity( x, Math.floor( pong.ball.yo + pong.ball.radius / 2 ), mult*500, 0 );		

		ball_counter++;

		if ( ball_counter == 12 ) {
	
			pong.ball.out = false;
			ball_counter = 0;
			
		}
	}

}

function jiggleBall(player, distance){

	pong.ball.x = player.x + distance + Math.random();
	pong.ball.y = player.y + player.height / 2 + Math.random();
	pong.ball.vx = 0;
	pong.ball.vy = 0;	

}

function prepareFrame(field) {

	if ( fps == 60){

		fps = 0;

	}

	fps++;

	colors.rotate();

	pong.player.color = colors.colors[0];
	pong.ai.color = colors.colors[1];
	pong.ball.color = colors.colors[2];

	field.setDensityRGB( Math.floor( pong.ball.x + pong.ball.radius / 2  ) , Math.floor( pong.ball.y + pong.ball.radius / 2 ), pong.ball.color );

	push(pong.player, field, 50);
	push(pong.ai, field, -50);

	explode(field);

	
	if ( pong.player.suck ) {			

		if ( !suck_on && suck_counter_1 > 30 ) {

			suck_on = true;

		}

		// make proportional power

		if ( suck_on ) {

			var straight_line_dist = pong.distance(pong.player, pong.ball );

			if ( Math.abs( straight_line_dist ) < 20 ) {

				ball_caught = true;

				jiggleBall( pong.player, 10);

				suck_counter_1--;				

				if ( suck_counter_1 == 0 ){

					field.setVelocity( 0, Math.floor( pong.player.y + pong.player.height / 2 ), 5000, 0 );				
					field.setDensityRGB( 0, Math.floor( pong.player.y + pong.player.height / 2 ), pong.player.color );			

					suck_on = false;
					ball_caught = false;

				}					

			}		

		}

	}			

	if ( suck_on && !pong.player.suck && ball_caught ){

		field.setVelocity( 0, Math.floor( pong.player.y + pong.player.height / 2 ), 5000, 0 );				
		field.setDensityRGB( 0, Math.floor( pong.player.y + pong.player.height / 2 ), pong.player.color );

		suck_on = false;		
		ball_caught = false;


	}

	if ( suck_counter_1 < 100 && fps % 10 == 1 )	{

		suck_counter_1 += 2;

	}

	if ( suck_counter_2 < 100 && fps % 10 == 1 )	{

		suck_counter_2 += 2;

	}	

	if ( !pong.ai.multiplayer ){

		if ( suck_counter_2 >= 90 ){


			pong.ai.suck = true;

		}

	}

	if ( pong.ai.suck ) {			

		if ( !suck_on2 && suck_counter_2 > 30 ) {

			suck_on2 = true;

		}

		// make proportional power

		if ( suck_on2 ) {

			var straight_line_dist = pong.distance(pong.ai, pong.ball );

			var aval = 20;

			if ( !pong.ai.multiplayer ){

				aval = 0;

			}			

			if ( Math.abs( straight_line_dist ) < aval  ) {

				ball_caught2 = true;

				jiggleBall( pong.ai, -10);

				suck_counter_2--;		

				var val = 0;		

				if ( !pong.ai.multiplayer ){

					val = 80;

				}

				if ( suck_counter_2 <= val ){

					if ( !pong.ai.multiplayer){
					field.setVelocity( Math.floor( pong.ai.x + pong.ai.width / 2 ), Math.floor( pong.ai.y + pong.ai.height / 2 ), -2500, 0 );	
				} else {
					field.setVelocity( Math.floor( pong.ai.x + pong.ai.width / 2 ), Math.floor( pong.ai.y + pong.ai.height / 2 ), -5000, 0 );	
				}
					field.setDensityRGB( Math.floor( pong.ai.x + pong.ai.width / 2  ) , Math.floor( pong.ai.y + pong.ai.height / 2 ), pong.ai.color );				

					suck_on2 = false;
					pong.ai.suck = false;
					ball_caught2 = false;

				}					

			}		

		}

	}		

	if ( suck_on2 && !pong.ai.suck && ball_caught2 ){

		field.setVelocity( Math.floor( pong.ai.x + pong.ai.width / 2 ), Math.floor( pong.ai.y + pong.ai.height / 2 ), -5000, 0 );	
		field.setDensityRGB( Math.floor( pong.ai.x + pong.ai.width / 2  ) , Math.floor( pong.ai.y + pong.ai.height / 2 ), pong.ai.color );				

		suck_on2 = false;		
		ball_caught2 = false;

	}

}

function drawPowerBar( color, x, y, width, height ){

	ctx.fillStyle = color;
	ctx.fillRect(x, y, width, height);

}

function drawSuck() {

	drawPowerBar( "black", 0, 1, canvas.width, 4 );
	drawPowerBar( arrayToRGBA( pong.ai.color ), 1,2, ( canvas.width/ 2 - 2 ) * suck_counter_1 / 100, 2 );
	drawPowerBar( arrayToRGBA( pong.player.color ), canvas.width / 2 + ( canvas.width/ 2 ) * ( 1 - suck_counter_2 / 100 ),2, ( canvas.width/ 2 - 1 ) * suck_counter_2 / 100, 2 );

}

function drawLives(){

	drawPowerBar( "black", 0,canvas.height - 5, canvas.width, 4);
	drawPowerBar( arrayToRGBA( pong.ai.color ), canvas.width / 2 + ( canvas.width/ 2 ) * ( 1 - pong.ai.life / 5 ),canvas.height - 4, ( canvas.width/ 2 - 1 ) * pong.ai.life / 5, 2);
	drawPowerBar( arrayToRGBA( pong.player.color ), 1,canvas.height - 4, ( canvas.width/ 2 - 2 ) * pong.player.life / 5, 2);

}

function switchAnimation() {

	
	if ( running ) {
		
		running = false;	
		document.getElementById("switch").innerHTML = "Unpause"


	} else {

		running = true;
		document.getElementById("switch").innerHTML = "Pause"

	}

	return;
	
}

function startAnimation() {
	
	running = true;

	return;
	
} 

// shim layer with setTimeout fallback
window.requestAnimFrame = (function(){
  return  window.requestAnimationFrame       || 
          window.webkitRequestAnimationFrame || 
          window.mozRequestAnimationFrame    || 
          window.oRequestAnimationFrame      || 
          window.msRequestAnimationFrame     || 
          function( callback ){
            window.setTimeout(callback, 1000 / FPS);
          };
})();

(function animloop(){

  requestAnimFrame(animloop);

  updateFrame();

})();

function Counter(){

	this.coul = 0;
	this.coul_incr = 0;

	this.symbols = [3,2,1, "GO!"];

	this.coul_switch = true;
	this.run_coul = true;
	this.cout_color = [];

	this.count_down = function(){

			this.coul++;

			if (this.cout_color.length == 0){
				this.cout_color = pong.ai.color;
			}

			if ( this.coul == 60 * 1 ){

				if (this.coul_incr % 2 == 0){

					this.cout_color = pong.player.color;
					
				} else {
					this.cout_color = pong.ai.color;
				}

				this.coul = 0;
				this.coul_incr++;

			}

		  	if ( this.coul_incr == 4 ){

	  			this.coul_incr = 0;
	  			this.run_coul = false;

	  			field.reset();

	  			pong.display = true;
	  			pong.player.suck = false;
	  			pong.init();  			

	  			pong.clear();
				
	  			return;

			}

			var half_width = canvas.width / 2 - 8;
			var half_height = canvas.width / 2 + 16;

			if ( this.coul_incr == 3 ){

				half_width -= 20;

			}


		  	ctx.font = "bold 34px Arial";
		  	ctx.fillStyle = "black";
		  	ctx.fillText(this.symbols[this.coul_incr], half_width - 1, half_height + 2);			

		  	ctx.fillStyle = arrayToRGBA( this.cout_color );
		  	ctx.font = "bold 32px Arial";
		  	ctx.fillText(this.symbols[this.coul_incr], half_width, half_height);	

	}	

}

function arrayToRGBA( a ){

	return "rgb(" + Math.floor( a[0] ) + "," + Math.floor( a[1] ) + "," + Math.floor( a[2] ) + ")";
}

function updateFrame() {
	
	if ( running ) {

		field.update();    
		
		pong.loop();
		
		notify();

		drawSuck();
		drawLives();

		if ( counter.run_coul ){

			counter.count_down();

		}

	}
	
}

function updateRes() {

		var r = 96;

		canvas.width = r;
		canvas.height = r;

		field.setResolution(r, r);
		pong.display = false;
        pong.init(); 

}

function key_check( e, keyMap, set) {

	var i;		

	for(i in keyMap) {

		if (keyMap.hasOwnProperty(i)) {
		
			if( e.keyCode === keyMap[i].code ) {
			
					keyMap[i].on = set;
					break;
					
				}
				
			}   
		}		
}

var keyDown = function(e) {

	key_check( e, pong.keyMap, true);

}

var keyUp = function(e) {

	key_check( e, pong.keyMap, false);

}

function begin() {

	field = new Fluid(canvas);
	field.setUICallback(prepareFrame);
	field.setDisplayFunction(field.toggleDisplayFunction(canvas, 0));

	pong = new Pong(canvas);
	colors = new Colors();
	counter = new Counter();

	window.addEventListener("keydown", keyDown, false);
	window.addEventListener("keyup", keyUp, false);
	
	updateRes();     
	startAnimation();

}

begin();